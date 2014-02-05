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

package connectivity

import (
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/bus/networkmanager"
	testingbus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
	"launchpad.net/ubuntu-push/util"
	"net/http/httptest"
	"testing"
	"time"
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type ConnSuite struct {
	timeouts []time.Duration
	log      logger.Logger
}

var _ = Suite(&ConnSuite{})

func (s *ConnSuite) SetUpSuite(c *C) {
	s.timeouts = util.SwapTimeouts([]time.Duration{0, 0, 0, 0})
}

func (s *ConnSuite) TearDownSuite(c *C) {
	util.SwapTimeouts(s.timeouts)
	s.timeouts = nil
}

func (s *ConnSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
	s.log.Debugf("---")
}

/*
   tests for connectedState's Start() method
*/

// when given a working config and bus, Start() will work
func (s *ConnSuite) TestStartWorks(c *C) {
	endp := testingbus.NewTestingEndpoint(condition.Work(true), condition.Work(true), uint32(networkmanager.Connecting))
	cs := connectedState{config: ConnectivityConfig{}, log: s.log, endp: endp}

	c.Check(cs.start(), Equals, networkmanager.Connecting)
}

// if the bus fails a couple of times, we're still OK
func (s *ConnSuite) TestStartRetriesConnect(c *C) {
	endp := testingbus.NewTestingEndpoint(condition.Fail2Work(2), condition.Work(true), uint32(networkmanager.Connecting))
	cs := connectedState{config: ConnectivityConfig{}, log: s.log, endp: endp}

	c.Check(cs.start(), Equals, networkmanager.Connecting)
	c.Check(cs.connAttempts, Equals, uint32(3)) // 1 more than the Fail2Work
}

// when the calls to NetworkManager fail for a bit, we're still OK
func (s *ConnSuite) TestStartRetriesCall(c *C) {
	endp := testingbus.NewTestingEndpoint(condition.Work(true), condition.Fail2Work(5), uint32(networkmanager.Connecting))
	cs := connectedState{config: ConnectivityConfig{}, log: s.log, endp: endp}

	c.Check(cs.start(), Equals, networkmanager.Connecting)

	c.Check(cs.connAttempts, Equals, uint32(6))
}

// when ... and bear with me ... the bus works, and the first call to
// get network manager's state works, but then you can't establish the
// watch, we recover and try again.
func (s *ConnSuite) TestStartRetriesWatch(c *C) {
	nmcond := condition.Chain(
		1, condition.Work(true), // 1 call to nm works
		1, condition.Work(false), // 1 call to nm fails
		0, condition.Work(true)) // and everything works from there on
	endp := testingbus.NewTestingEndpoint(condition.Work(true), nmcond,
		uint32(networkmanager.Connecting),
		uint32(networkmanager.ConnectedGlobal))
	cs := connectedState{config: ConnectivityConfig{}, log: s.log, endp: endp}

	c.Check(cs.start(), Equals, networkmanager.Connecting)
	c.Check(cs.connAttempts, Equals, uint32(2))
	c.Check(<-cs.networkStateCh, Equals, networkmanager.Connecting)
	c.Check(<-cs.networkStateCh, Equals, networkmanager.ConnectedGlobal)
}

/*
   tests for connectedStateStep()
*/

func (s *ConnSuite) TestSteps(c *C) {
	var webget_p condition.Interface = condition.Work(true)
	recheck_timeout := 50 * time.Millisecond

	cfg := ConnectivityConfig{
		RecheckTimeout: config.ConfigTimeDuration{recheck_timeout},
	}
	ch := make(chan networkmanager.State, 10)
	cs := &connectedState{
		config:         cfg,
		networkStateCh: ch,
		timer:          time.NewTimer(time.Second),
		log:            s.log,
		webget:         func(ch chan<- bool) { ch <- webget_p.OK() },
		lastSent:       false,
	}
	ch <- networkmanager.ConnectedGlobal
	f, e := cs.connectedStateStep()
	c.Check(e, IsNil)
	c.Check(f, Equals, true)
	ch <- networkmanager.ConnectedGlobal // a ConnectedGlobal when connected signals trouble
	f, e = cs.connectedStateStep()
	c.Check(e, IsNil)
	c.Check(f, Equals, false) // so we assume a disconnect happened
	f, e = cs.connectedStateStep()
	c.Check(e, IsNil)
	c.Check(f, Equals, true) // and if the web check works, go back to connected

	// same scenario, but with failing web check
	webget_p = condition.Fail2Work(1)
	ch <- networkmanager.ConnectedGlobal
	f, e = cs.connectedStateStep()
	c.Check(e, IsNil)
	c.Check(f, Equals, false) // first false is from assuming a Connected signals trouble

	// the next call to Step will time out
	_ch := make(chan bool, 1)
	_t := time.NewTimer(recheck_timeout / 2)

	go func() {
		f, e := cs.connectedStateStep()
		c.Check(e, IsNil)
		_ch <- f
	}()

	select {
	case <-_ch:
		c.Fatal("test failed to timeout")
	case <-_t.C:
	}

	// now an recheckTimeout later, we'll get true
	c.Check(<-_ch, Equals, true)

	ch <- networkmanager.Disconnected    // this should trigger a 'false'
	ch <- networkmanager.Disconnected    // this should not
	ch <- networkmanager.ConnectedGlobal // this should trigger a 'true'

	f, e = cs.connectedStateStep()
	c.Check(e, IsNil)
	c.Check(f, Equals, false)
	f, e = cs.connectedStateStep()
	c.Check(e, IsNil)
	c.Check(f, Equals, true)

	close(ch) // this should make it error out
	_, e = cs.connectedStateStep()
	c.Check(e, NotNil)
}

/*
   tests for ConnectedState()
*/

// yes, this is an integration test
func (s *ConnSuite) TestRun(c *C) {
	ts := httptest.NewServer(mkHandler(staticText))
	defer ts.Close()

	cfg := ConnectivityConfig{
		ConnectivityCheckURL: ts.URL,
		ConnectivityCheckMD5: staticHash,
		RecheckTimeout:       config.ConfigTimeDuration{time.Second},
	}

	endp := testingbus.NewTestingEndpoint(condition.Work(true), condition.Work(true),
		uint32(networkmanager.ConnectedGlobal),
		uint32(networkmanager.ConnectedGlobal),
		uint32(networkmanager.Disconnected),
	)

	watchTicker := make(chan bool)
	testingbus.SetWatchTicker(endp, watchTicker)

	out := make(chan bool)
	dt := time.Second / 10
	timer := time.NewTimer(dt)
	go ConnectedState(endp, cfg, s.log, out)
	var v bool
	expecteds := []struct {
		p bool
		s string
		n int
	}{
		{false, "first state is always false", 0},
		{true, "then it should be true as per ConnectedGlobal above", 0},
		{false, "then, false (upon receiving the next ConnectedGlobal)", 1},
		{true, "then it should be true (webcheck passed)", 0},
		{false, "then it should be false (Disconnected)", 1},
		{false, "then it should be false again because it's restarted", 1},
	}

	for i, expected := range expecteds {
		for j := 0; j < expected.n; j++ {
			watchTicker <- true
		}
		timer.Reset(dt)
		select {
		case v = <-out:
			break
		case <-timer.C:
			c.Fatalf("Timed out before getting value (#%d: %s)", i+1, expected.s)
		}

		c.Check(v, Equals, expected.p, Commentf(expected.s))
	}
}
