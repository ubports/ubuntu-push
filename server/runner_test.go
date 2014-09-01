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

package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/config"
	helpers "launchpad.net/ubuntu-push/testing"
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
	runner := HTTPServeRunner(nil, h, &testHTTPServeParsedConfig, nil)
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
	TLSParsedConfig: TLSParsedConfig{
		ParsedKeyPEMFile:  "",
		ParsedCertPEMFile: "",
		keyPEMBlock:       helpers.TestKeyPEMBlock,
		certPEMBlock:      helpers.TestCertPEMBlock,
	},
}

func (s *runnerSuite) TestDevicesRunner(c *C) {
	prevBootLogger := BootLogger
	testlog := helpers.NewTestLogger(c, "debug")
	BootLogger = testlog
	defer func() {
		BootLogger = prevBootLogger
	}()
	runner := DevicesRunner(nil, func(conn net.Conn) error { return nil }, BootLogger, &testDevicesParsedConfig)
	c.Assert(s.lst, Not(IsNil))
	s.lst.Close()
	c.Check(s.kind, Equals, "devices")
	c.Check(runner, PanicMatches, "accepting device connections:.*closed.*")
}

func (s *runnerSuite) TestDevicesRunnerAdoptListener(c *C) {
	prevBootLogger := BootLogger
	testlog := helpers.NewTestLogger(c, "debug")
	BootLogger = testlog
	defer func() {
		BootLogger = prevBootLogger
	}()
	lst0, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, IsNil)
	defer lst0.Close()
	DevicesRunner(lst0, func(conn net.Conn) error { return nil }, BootLogger, &testDevicesParsedConfig)
	c.Assert(s.lst, Not(IsNil))
	c.Check(s.lst.Addr().String(), Equals, lst0.Addr().String())
	s.lst.Close()
}

func (s *runnerSuite) TestHTTPServeRunnerAdoptListener(c *C) {
	lst0, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, IsNil)
	defer lst0.Close()
	HTTPServeRunner(lst0, nil, &testHTTPServeParsedConfig, nil)
	c.Assert(s.lst, Equals, lst0)
	c.Check(s.kind, Equals, "http")
}

func (s *runnerSuite) TestHTTPServeRunnerTLS(c *C) {
	errCh := make(chan interface{}, 1)
	h := http.HandlerFunc(testHandle)
	runner := HTTPServeRunner(nil, h, &testHTTPServeParsedConfig, helpers.TestTLSServerConfig)
	c.Assert(s.lst, Not(IsNil))
	defer s.lst.Close()
	c.Check(s.kind, Equals, "http")
	go func() {
		defer func() {
			errCh <- recover()
		}()
		runner()
	}()
	cp := x509.NewCertPool()
	ok := cp.AppendCertsFromPEM(helpers.TestCertPEMBlock)
	c.Assert(ok, Equals, true)
	cli := http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			RootCAs:    cp,
			ServerName: "push-delivery",
		}},
	}
	resp, err := cli.Get(fmt.Sprintf("https://%s/", s.lst.Addr()))
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, 200)
	body, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	c.Check(string(body), Equals, "yay!\n")
	s.lst.Close()
	c.Check(<-errCh, Matches, "accepting http connections:.*closed.*")
}
