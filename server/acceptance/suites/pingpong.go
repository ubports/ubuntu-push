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

package suites

import (
	"runtime"
	"strings"
	"time"

	. "launchpad.net/gocheck"
)

// PingPongAcceptanceSuite has tests about connectivity and ping-pong requests.
type PingPongAcceptanceSuite struct {
	AcceptanceSuite
}

// Tests about connection, ping-pong, disconnection scenarios

func (s *PingPongAcceptanceSuite) TestConnectPingPing(c *C) {
	errCh := make(chan error, 1)
	events := make(chan string, 10)
	sess := testClientSession(s.ServerAddr, "DEVA", "m1", "img1", true)
	err := sess.Dial()
	c.Assert(err, IsNil)
	intercept := func(ic *interceptingConn, op string, b []byte) (bool, int, error) {
		// would be 3rd ping read, based on logged traffic
		if op == "read" && ic.totalRead >= 79 {
			// exit the sess.Run() goroutine, client will close
			runtime.Goexit()
		}
		return false, 0, nil
	}
	sess.Connection = &interceptingConn{sess.Connection, 0, 0, intercept}
	go func() {
		errCh <- sess.Run(events)
	}()
	connectCli := NextEvent(events, errCh)
	connectSrv := NextEvent(s.ServerEvents, nil)
	registeredSrv := NextEvent(s.ServerEvents, nil)
	tconnect := time.Now()
	c.Assert(connectSrv, Matches, ".*session.* connected .*")
	c.Assert(registeredSrv, Matches, ".*session.* registered DEVA")
	c.Assert(strings.HasSuffix(connectSrv, connectCli), Equals, true)
	c.Assert(NextEvent(events, errCh), Equals, "Ping")
	elapsedOfPing := float64(time.Since(tconnect)) / float64(500*time.Millisecond)
	c.Check(elapsedOfPing >= 1.0, Equals, true)
	c.Check(elapsedOfPing < 1.05, Equals, true)
	c.Assert(NextEvent(events, errCh), Equals, "Ping")
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, ".*session.* ended with: EOF")
	c.Check(len(errCh), Equals, 0)
}

func (s *PingPongAcceptanceSuite) TestConnectPingNeverPong(c *C) {
	errCh := make(chan error, 1)
	events := make(chan string, 10)
	sess := testClientSession(s.ServerAddr, "DEVB", "m1", "img1", true)
	err := sess.Dial()
	c.Assert(err, IsNil)
	intercept := func(ic *interceptingConn, op string, b []byte) (bool, int, error) {
		// would be pong to 2nd ping, based on logged traffic
		if op == "write" && ic.totalRead >= 67 {
			time.Sleep(200 * time.Millisecond)
			// exit the sess.Run() goroutine, client will close
			runtime.Goexit()
		}
		return false, 0, nil
	}
	sess.Connection = &interceptingConn{sess.Connection, 0, 0, intercept}
	go func() {
		errCh <- sess.Run(events)
	}()
	c.Assert(NextEvent(events, errCh), Matches, "connected .*")
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, ".*session.* connected .*")
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, ".*session.* registered .*")
	c.Assert(NextEvent(events, errCh), Equals, "Ping")
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*timeout`)
	c.Check(len(errCh), Equals, 0)
}
