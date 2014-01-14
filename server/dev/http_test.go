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
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	// "log"
	"net"
	"net/http"
	"time"
)

type httpSuite struct{}

var _ = Suite(&httpSuite{})

type testHTTPServeConfig struct{}

func (cfg testHTTPServeConfig) HTTPAddr() string {
	return "127.0.0.1:0"
}

func (cfg testHTTPServeConfig) HTTPReadTimeout() time.Duration {
	return 5 * time.Second
}

func (cfg testHTTPServeConfig) HTTPWriteTimeout() time.Duration {
	return 5 * time.Second
}

func testHandle(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "yay!\n")
}

func (s *httpSuite) TestRunHTTPServe(c *C) {
	cfg := testHTTPServeConfig{}
	lst, err := net.Listen("tcp", cfg.HTTPAddr())
	c.Assert(err, IsNil)
	defer lst.Close()
	errCh := make(chan error, 1)
	h := http.HandlerFunc(testHandle)
	go func() {
		errCh <- RunHTTPServe(lst, h, cfg)
	}()
	resp, err := http.Get(fmt.Sprintf("http://%s/", lst.Addr()))
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, 200)
	body, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	c.Check(string(body), Equals, "yay!\n")
	lst.Close()
	c.Check(<-errCh, ErrorMatches, ".*closed.*")
}
