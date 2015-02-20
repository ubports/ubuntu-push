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
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"launchpad.net/go-dbus/v1"
	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/networkmanager"
	testingbus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
	"launchpad.net/ubuntu-push/util"
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

// when the calls to NetworkManager fails for a bit, we're still OK
func (s *ConnSuite) TestStartRetriesCall(c *C) {
	endp := testingbus.NewTestingEndpoint(condition.Work(true), condition.Fail2Work(5), uint32(networkmanager.Connecting))
	cs := connectedState{config: ConnectivityConfig{}, log: s.log, endp: endp}

	c.Check(cs.start(), Equals, networkmanager.Connecting)

	c.Check(cs.connAttempts, Equals, uint32(6))
}

// when some of the calls to NetworkManager fails for a bit, we're still OK
func (s *ConnSuite) TestStartRetriesCall2(c *C) {
	cond := condition.Chain(3, condition.Work(true), 1, condition.Work(false),
		1, condition.Work(true))

	endp := testingbus.NewTestingEndpoint(condition.Work(true), cond, uint32(networkmanager.Connecting))
	cs := connectedState{config: ConnectivityConfig{}, log: s.log, endp: endp}

	c.Check(cs.start(), Equals, networkmanager.Connecting)
}

// when... and bear with me... the bus works, and the first call to
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

// a racyEndpoint is an endpoint that behaves differently depending on
// how much time passes between getting the state and setting up the
// watch
type racyEndpoint struct {
	stateGot bool
	maxTime  time.Time
	delta    time.Duration
	lock     sync.RWMutex
}

func (rep *racyEndpoint) GetProperty(prop string) (interface{}, error) {
	switch prop {
	case "state":
		rep.lock.Lock()
		defer rep.lock.Unlock()
		rep.stateGot = true
		rep.maxTime = time.Now().Add(rep.delta)
		return uint32(networkmanager.Connecting), nil
	case "PrimaryConnection":
		return dbus.ObjectPath("/something"), nil
	default:
		return nil, nil
	}
}

func (rep *racyEndpoint) WatchSignal(member string, f func(...interface{}), d func()) error {
	if member == "StateChanged" {
		// we count never having gotten the state as happening "after" now.
		rep.lock.RLock()
		defer rep.lock.RUnlock()
		ok := !rep.stateGot || time.Now().Before(rep.maxTime)
		go func() {
			if ok {
				f(uint32(networkmanager.ConnectedGlobal))
			}
			d()
		}()
	}
	return nil
}

func (*racyEndpoint) Close()                                                        {}
func (*racyEndpoint) Dial() error                                                   { return nil }
func (*racyEndpoint) String() string                                                { return "racyEndpoint" }
func (*racyEndpoint) Call(string, []interface{}, ...interface{}) error              { return nil }
func (*racyEndpoint) GrabName(bool) <-chan error                                    { return nil }
func (*racyEndpoint) WatchMethod(bus.DispatchMap, string, ...interface{})           {}
func (*racyEndpoint) Signal(member string, suffix string, args []interface{}) error { return nil }
func (*racyEndpoint) SetProperty(string, string, interface{}) error                 { return nil }

var _ bus.Endpoint = (*racyEndpoint)(nil)

// takeNext takes a value from given channel with a 1s timeout
func takeNext(ch <-chan networkmanager.State) networkmanager.State {
	select {
	case <-time.After(time.Second):
		panic("channel stuck: too long waiting")
	case v := <-ch:
		return v
	}
}

// test that if the nm state goes from connecting to connected very
// shortly after calling GetState, we don't lose the event.
func (s *ConnSuite) TestStartAvoidsRace(c *C) {
	for delta := time.Second; delta > 1; delta /= 2 {
		rep := &racyEndpoint{delta: delta}
		cs := connectedState{config: ConnectivityConfig{}, log: s.log, endp: rep}
		f := Commentf("when delta=%s", delta)
		c.Assert(cs.start(), Equals, networkmanager.Connecting, f)
		c.Assert(takeNext(cs.networkStateCh), Equals, networkmanager.ConnectedGlobal, f)
	}
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
	ch <- networkmanager.Disconnected
	ch <- networkmanager.ConnectedGlobal
	f, e = cs.connectedStateStep()
	c.Check(e, IsNil)
	c.Check(f, Equals, false)
	f, e = cs.connectedStateStep()
	c.Check(e, IsNil)
	c.Check(f, Equals, true)

	// same scenario, but with failing web check
	webget_p = condition.Fail2Work(1)
	ch <- networkmanager.Disconnected
	ch <- networkmanager.ConnectedGlobal
	f, e = cs.connectedStateStep()
	c.Check(e, IsNil)
	c.Check(f, Equals, false) // first false is from the Disconnected

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
		uint32(networkmanager.Disconnected),
	)

	watchTicker := make(chan []interface{})
	testingbus.SetWatchSource(endp, "StateChanged", watchTicker)
	nopTicker := make(chan []interface{})
	testingbus.SetWatchSource(endp, "PropertiesChanged", nopTicker)
	states := [][]interface{}{
		[]interface{}{uint32(networkmanager.ConnectedGlobal)},
		[]interface{}{uint32(networkmanager.Disconnected)},
	}

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
		{false, "first state is always false", -1},
		{true, "then it should be true as per ConnectedGlobal above", 0},
		{false, "then it should be false (Disconnected)", 1},
		{false, "then it should be false again because it's restarted", -2},
	}

	defer func() {
		if watchTicker != nil {
			close(watchTicker)
		}
	}()
	defer close(nopTicker)
	for i, expected := range expecteds {
		// xxx ugly
		if expected.n >= 0 {
			watchTicker <- states[expected.n]
		} else if expected.n == -2 {
			close(watchTicker)
			watchTicker = nil
		}
		timer.Reset(dt)
		select {
		case v = <-out:
			break
		case <-timer.C:
			c.Fatalf("Timed out before getting value (#%d: %s)", i+1, expected.s)
		}
		c.Assert(v, Equals, expected.p, Commentf(expected.s))
	}
}

func (s *ConnSuite) TestRun4Active(c *C) {
	ts := httptest.NewServer(mkHandler(staticText))
	defer ts.Close()

	cfg := ConnectivityConfig{
		ConnectivityCheckURL: ts.URL,
		ConnectivityCheckMD5: staticHash,
		RecheckTimeout:       config.ConfigTimeDuration{time.Second},
	}

	endp := testingbus.NewTestingEndpoint(condition.Work(true), condition.Work(true),
		uint32(networkmanager.ConnectedGlobal),
		map[string]dbus.Variant{"PrimaryConnection": dbus.Variant{dbus.ObjectPath("hello")}},
	)

	watchTicker := make(chan []interface{})
	testingbus.SetWatchSource(endp, "PropertiesChanged", watchTicker)
	nopTicker := make(chan []interface{})
	testingbus.SetWatchSource(endp, "StateChanged", nopTicker)

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
		{false, "then, false (PrimaryConnection changed)", 2},
		{true, "then it should be true (webcheck passed)", 0},
	}

	defer func() {
		if watchTicker != nil {
			close(watchTicker)
		}
	}()
	defer close(nopTicker)
	for i, expected := range expecteds {
		if expected.n != 0 {
			watchTicker <- []interface{}{
				map[string]dbus.Variant{"PrimaryConnection": dbus.Variant{dbus.ObjectPath("hello")}},
			}
		}
		timer.Reset(dt)
		select {
		case v = <-out:
			break
		case <-timer.C:
			c.Fatalf("Timed out before getting value (#%d: %s)", i+1, expected.s)
		}
		c.Assert(v, Equals, expected.p, Commentf(expected.s))
	}
}
