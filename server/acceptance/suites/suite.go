/*
 Copyright 2013-2014 Canonical Ltd.

 This program is free software: you can redistribute it and/or modify it
 under the terms of the GNU General Public License version 3, as published
 by the Free Software Foundation.

 This program is distributed in the hope that it will be useful, but
 WITHOUT ANY WARRANTY; without even the implied warranties of
 MERCHANTABILITY, SATISFACTORY QUALITY, or FITNESS FOR A PARTICULAR
 PURPOSE.  See the GNU General Public License for more details.

 You should have received a copy of the GNU General Public License along
 with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

// Package suites contains reusable acceptance test suites.
package suites

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"runtime"
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/server/acceptance"
	helpers "launchpad.net/ubuntu-push/testing"
)

// AcceptanceSuite has the basic functionality of the acceptance suites.
type AcceptanceSuite struct {
	// hook to start the server(s)
	StartServer func(c *C) (logs <-chan string, kill func(), serverAddr, apiURL string)
	// running bits
	serverKill   func()
	serverAddr   string
	serverAPIURL string
	serverEvents <-chan string
	httpClient   *http.Client
}

// Start a new server for each test.
func (s *AcceptanceSuite) SetUpTest(c *C) {
	logs, kill, addr, url := s.StartServer(c)
	s.serverEvents = logs
	s.serverKill = kill
	s.serverAddr = addr
	s.serverAPIURL = url
	s.httpClient = &http.Client{}
}

func (s *AcceptanceSuite) TearDownTest(c *C) {
	if s.serverKill != nil {
		s.serverKill()
	}
}

// Post a request.
func (s *AcceptanceSuite) postRequest(path string, message interface{}) (string, error) {
	packedMessage, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}
	reader := bytes.NewReader(packedMessage)

	url := s.serverAPIURL + path
	request, _ := http.NewRequest("POST", url, reader)
	request.ContentLength = int64(reader.Len())
	request.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body), err
}

func testClientSession(addr string, deviceId string, reportPings bool) *acceptance.ClientSession {
	certPEMBlock, err := ioutil.ReadFile(helpers.SourceRelative("../config/testing.cert"))
	if err != nil {
		panic(fmt.Sprintf("could not read config/testing.cert: %v", err))
	}
	return &acceptance.ClientSession{
		ExchangeTimeout: 100 * time.Millisecond,
		ServerAddr:      addr,
		CertPEMBlock:    certPEMBlock,
		DeviceId:        deviceId,
		ReportPings:     reportPings,
	}
}

// typically combined with -gocheck.vv or test selection
var logTraffic = flag.Bool("logTraffic", false, "log traffic")

type connInterceptor func(ic *interceptingConn, op string, b []byte) (bool, int, error)

type interceptingConn struct {
	net.Conn
	totalRead    int
	totalWritten int
	intercept    connInterceptor
}

func (ic *interceptingConn) Write(b []byte) (n int, err error) {
	done := false
	before := ic.totalWritten
	if ic.intercept != nil {
		done, n, err = ic.intercept(ic, "write", b)
	}
	if !done {
		n, err = ic.Conn.Write(b)
	}
	ic.totalWritten += n
	if *logTraffic {
		fmt.Printf("W[%v]: %d %#v %v %d\n", ic.Conn.LocalAddr(), before, string(b[:n]), err, ic.totalWritten)
	}
	return
}

func (ic *interceptingConn) Read(b []byte) (n int, err error) {
	done := false
	before := ic.totalRead
	if ic.intercept != nil {
		done, n, err = ic.intercept(ic, "read", b)
	}
	if !done {
		n, err = ic.Conn.Read(b)
	}
	ic.totalRead += n
	if *logTraffic {
		fmt.Printf("R[%v]: %d %#v %v %d\n", ic.Conn.LocalAddr(), before, string(b[:n]), err, ic.totalRead)
	}
	return
}

// Start a client.
func (s *AcceptanceSuite) startClient(c *C, devId string, levels map[string]int64) (events <-chan string, errorCh <-chan error, stop func()) {
	errCh := make(chan error, 1)
	cliEvents := make(chan string, 10)
	sess := testClientSession(s.serverAddr, devId, false)
	sess.Levels = levels
	err := sess.Dial()
	c.Assert(err, IsNil)
	clientShutdown := make(chan bool, 1) // abused as an atomic flag
	intercept := func(ic *interceptingConn, op string, b []byte) (bool, int, error) {
		// read after ack
		if op == "read" && len(clientShutdown) > 0 {
			// exit the sess.Run() goroutine, client will close
			runtime.Goexit()
		}
		return false, 0, nil
	}
	sess.Connection = &interceptingConn{sess.Connection, 0, 0, intercept}
	go func() {
		errCh <- sess.Run(cliEvents)
	}()
	c.Assert(NextEvent(cliEvents, errCh), Matches, "connected .*")
	c.Assert(NextEvent(s.serverEvents, nil), Matches, ".*session.* connected .*")
	c.Assert(NextEvent(s.serverEvents, nil), Matches, ".*session.* registered "+devId)
	return cliEvents, errCh, func() { clientShutdown <- true }
}
