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

// ServerHandle holds the information to attach a client to the test server.
type ServerHandle struct {
	ServerAddr   string
	ServerEvents <-chan string
}

// Start a client.
func (h *ServerHandle) StartClient(c *C, devId string, levels map[string]int64) (events <-chan string, errorCh <-chan error, stop func()) {
	errCh := make(chan error, 1)
	cliEvents := make(chan string, 10)
	sess := testClientSession(h.ServerAddr, devId, false)
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
	c.Assert(NextEvent(h.ServerEvents, nil), Matches, ".*session.* connected .*")
	c.Assert(NextEvent(h.ServerEvents, nil), Matches, ".*session.* registered "+devId)
	return cliEvents, errCh, func() { clientShutdown <- true }
}

// AcceptanceSuite has the basic functionality of the acceptance suites.
type AcceptanceSuite struct {
	// hook to start the server(s)
	StartServer func(c *C, s *AcceptanceSuite) (logs <-chan string, serverAddr, apiURL string)
	// runtime information
	ServerHandle
	ServerAPIURL string
	// KillGroup should be populated by StartServer with functions
	// to kill the server process
	KillGroup map[string]func()
	// other state
	httpClient *http.Client
}

// Start a new server for each test.
func (s *AcceptanceSuite) SetUpTest(c *C) {
	s.KillGroup = make(map[string]func())
	logs, addr, url := s.StartServer(c, s)
	s.ServerEvents = logs
	s.ServerAddr = addr
	s.ServerAPIURL = url
	s.httpClient = &http.Client{}
}

func (s *AcceptanceSuite) TearDownTest(c *C) {
	for _, f := range s.KillGroup {
		f()
	}
}

// Post a API request.
func (s *AcceptanceSuite) PostRequest(path string, message interface{}) (string, error) {
	packedMessage, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}
	reader := bytes.NewReader(packedMessage)

	url := s.ServerAPIURL + path
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
