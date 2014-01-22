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

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"net"
	"net/http"
	"time"
)

type runnerSuite struct {
	prevBootLogListener func(string, net.Listener)
	prevBootLogFatalf   func(string, ...interface{})
	lst                 net.Listener
	kind                string
}

var _ = Suite(&runnerSuite{})

func (s *runnerSuite) SetUpSuite(c *C) {
	s.prevBootLogFatalf = BootLogFatalf
	s.prevBootLogListener = BootLogListener
	BootLogFatalf = func(format string, v ...interface{}) {
		panic(fmt.Sprintf(format, v...))
	}
	BootLogListener = func(kind string, lst net.Listener) {
		s.kind = kind
		s.lst = lst
	}
}

func (s *runnerSuite) TearDownSuite(c *C) {
	BootLogListener = s.prevBootLogListener
	BootLogFatalf = s.prevBootLogFatalf
}

var testHTTPServeParsedConfig = HTTPServeParsedConfig{
	"127.0.0.1:0",
	config.ConfigTimeDuration{5 * time.Second},
	config.ConfigTimeDuration{5 * time.Second},
}

func testHandle(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "yay!\n")
}

func (s *runnerSuite) TestHTTPServeRunner(c *C) {
	errCh := make(chan interface{}, 1)
	h := http.HandlerFunc(testHandle)
	runner := HTTPServeRunner(h, &testHTTPServeParsedConfig)
	c.Assert(s.lst, Not(IsNil))
	defer s.lst.Close()
	c.Check(s.kind, Equals, "http")
	go func() {
		defer func() {
			errCh <- recover()
		}()
		runner()
	}()
	resp, err := http.Get(fmt.Sprintf("http://%s/", s.lst.Addr()))
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, 200)
	body, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	c.Check(string(body), Equals, "yay!\n")
	s.lst.Close()
	c.Check(<-errCh, Matches, "accepting http connections:.*closed.*")
}

var testDevicesParsedConfig = DevicesParsedConfig{
	ParsedPingInterval:     config.ConfigTimeDuration{60 * time.Second},
	ParsedExchangeTimeout:  config.ConfigTimeDuration{10 * time.Second},
	ParsedBrokerQueueSize:  config.ConfigQueueSize(1000),
	ParsedSessionQueueSize: config.ConfigQueueSize(10),
	ParsedAddr:             "127.0.0.1:0",
	ParsedKeyPEMFile:       "",
	ParsedCertPEMFile:      "",
	keyPEMBlock:            helpers.TestKeyPEMBlock,
	certPEMBlock:           helpers.TestCertPEMBlock,
}

func (s *runnerSuite) TestDevicesRunner(c *C) {
	buf := &bytes.Buffer{}
	prevBootLogger := BootLogger
	BootLogger = logger.NewSimpleLogger(buf, "debug")
	defer func() {
		BootLogger = prevBootLogger
	}()
	runner := DevicesRunner(func(conn net.Conn) error { return nil }, BootLogger, &testDevicesParsedConfig)
	c.Assert(s.lst, Not(IsNil))
	s.lst.Close()
	c.Check(s.kind, Equals, "devices")
	c.Check(runner, PanicMatches, "accepting device connections:.*closed.*")
}
