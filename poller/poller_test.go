/*
 Copyright 2014-2015 Canonical Ltd.

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
package poller

import (
	"testing"
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/bus/networkmanager"
	"launchpad.net/ubuntu-push/client/session"
	helpers "launchpad.net/ubuntu-push/testing"
)

// hook up gocheck
func TestPoller(t *testing.T) { TestingT(t) }

type PrSuite struct {
	log *helpers.TestLogger
	myd *myD
}

var _ = Suite(&PrSuite{})

type myD struct {
	// in/out for RequestWakeup
	reqWakeName   string
	reqWakeTime   time.Time
	reqWakeCookie string
	reqWakeErr    error
	// WatchWakeups
	watchWakeCh  chan bool
	watchWakeErr error
	// RequestWakelock
	reqLockName   string
	reqLockCookie string
	reqLockErr    error
	// ClearWakelock
	clearLockCookie string
	clearLockErr    error
	// Poll
	pollErr error
	// WatchDones
	watchDonesCh  <-chan bool
	watchDonesErr error
	// State
	stateState session.ClientSessionState
}

func (m *myD) RequestWakeup(name string, wakeupTime time.Time) (string, error) {
	m.reqWakeName = name
	m.reqWakeTime = wakeupTime
	time.AfterFunc(100*time.Millisecond, func() {
		m.watchWakeCh <- true
	})
	return m.reqWakeCookie, m.reqWakeErr
}
func (m *myD) RequestWakelock(name string) (string, error) {
	m.reqLockName = name
	return m.reqLockCookie, m.reqLockErr
}
func (m *myD) ClearWakelock(cookie string) error {
	m.clearLockCookie = cookie
	return m.clearLockErr
}
func (m *myD) ClearWakeup(cookie string) error {
	m.watchWakeCh <- false
	return nil
}
func (m *myD) WatchWakeups() (<-chan bool, error) { return m.watchWakeCh, m.watchWakeErr }
func (m *myD) Poll() error                        { return m.pollErr }
func (m *myD) WatchDones() (<-chan bool, error)   { return m.watchDonesCh, m.watchDonesErr }
func (m *myD) State() session.ClientSessionState  { return m.stateState }

func (s *PrSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
	s.myd = &myD{}
}

const (
	connectedGlobal    = networkmanager.ConnectedGlobal
	disconnectedGlobal = networkmanager.Disconnected
)

func (s *PrSuite) TestStep(c *C) {
	p := &poller{
		times:                Times{},
		log:                  s.log,
		powerd:               s.myd,
		polld:                s.myd,
		sessionState:         s.myd,
		requestWakeupCh:      make(chan struct{}),
		requestedWakeupErrCh: make(chan error),
		holdsWakeLockCh:      make(chan bool),
	}
	s.myd.reqLockCookie = "wakelock cookie"
	s.myd.stateState = session.Running
	wakeupCh := make(chan bool, 1)
	s.myd.watchWakeCh = wakeupCh
	// we won't get the "done" signal in time ;)
	doneCh := make(chan bool)
	// and a channel to get the return value from a goroutine
	ch := make(chan string)
	// now, run
	filteredWakeUpCh := make(chan bool)
	go p.control(wakeupCh, filteredWakeUpCh, connectedGlobal, nil,)
	go func() { ch <- p.step(filteredWakeUpCh, doneCh, "old cookie") }()
	select {
	case s := <-ch:
		c.Check(s, Equals, "wakelock cookie")
	case <-time.After(time.Second):
		c.Fatal("timeout waiting for step")
	}
	// check we cleared the old cookie
	c.Check(s.myd.clearLockCookie, Equals, "old cookie")
}

func (s *PrSuite) TestControl(c *C) {
	p := &poller{
		times:                Times{},
		log:                  s.log,
		powerd:               s.myd,
		polld:                s.myd,
		sessionState:         s.myd,
		requestWakeupCh:      make(chan struct{}),
		requestedWakeupErrCh: make(chan error),
		holdsWakeLockCh:      make(chan bool),
	}
	wakeUpCh := make(chan bool)
	filteredWakeUpCh := make(chan bool)
	s.myd.watchWakeCh = make(chan bool, 1)
	nmStateCh := make(chan networkmanager.State)
	go p.control(wakeUpCh, filteredWakeUpCh, connectedGlobal, nmStateCh)

	// works
	err := p.requestWakeup()
	c.Assert(err, IsNil)
	c.Check(<-s.myd.watchWakeCh, Equals, true)

	// there's a wakeup already
	err = p.requestWakeup()
	c.Assert(err, IsNil)
	c.Check(s.myd.watchWakeCh, HasLen, 0)

	// wakeup happens
	wakeUpCh <- true
	<-filteredWakeUpCh

    nmStateCh <- disconnectedGlobal
    err = p.requestWakeup()
    c.Assert(err, IsNil)
    c.Check(s.myd.watchWakeCh, HasLen, 0)

	// connected
	nmStateCh <- connectedGlobal
	c.Check(<-s.myd.watchWakeCh, Equals, true)

	// disconnected
	nmStateCh <- disconnectedGlobal
	// pending wakeup was cleared
	c.Check(<-s.myd.watchWakeCh, Equals, false)

}
