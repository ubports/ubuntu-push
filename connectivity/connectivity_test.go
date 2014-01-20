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
	"io/ioutil"
	. "launchpad.net/gocheck"
	testingbus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/networkmanager"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/testing/condition"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type ConnSuite struct{}

var _ = Suite(&ConnSuite{})

var nullog = logger.NewSimpleLogger(ioutil.Discard, "error")

/*
   tests for connectedState's ConnectTimeout() method
*/

// When given no timeouts, ConnectTimeout() returns 0 forever
func (s *ConnSuite) TestConnectTimeoutWorksWithNoTimeouts(c *C) {
	cs := connectedState{}
	c.Check(cs.ConnectTimeout(), Equals, time.Duration(0))
	c.Check(cs.ConnectTimeout(), Equals, time.Duration(0))
}

// when given a few timeouts, ConnectTimeout() returns them each in
// turn, and then repeats the last one
func (s *ConnSuite) TestConnectTimeoutWorks(c *C) {
	ts := []config.ConfigTimeDuration{
		config.ConfigTimeDuration{0},
		config.ConfigTimeDuration{2 * time.Second},
		config.ConfigTimeDuration{time.Second},
	}
	cs := connectedState{config: Config{ConnectTimeouts: ts}}
	c.Check(cs.ConnectTimeout(), Equals, time.Duration(0))
	c.Check(cs.ConnectTimeout(), Equals, 2*time.Second)
	c.Check(cs.ConnectTimeout(), Equals, time.Second)
	c.Check(cs.ConnectTimeout(), Equals, time.Second)
	c.Check(cs.ConnectTimeout(), Equals, time.Second)
	c.Check(cs.ConnectTimeout(), Equals, time.Second)
	// ... ad nauseam
}

/*
   tests for connectedState's Start() method
*/

// when given a working config and bus, Start() will work
func (s *ConnSuite) TestStartWorks(c *C) {
	cfg := Config{}
	tb := testingbus.New(condition.Work(true), condition.Work(true), uint32(networkmanager.Connecting))
	cs := connectedState{config: cfg, log: nullog, bus: tb}

	c.Check(cs.Start(), Equals, networkmanager.Connecting)
}

// if the bus fails a couple of times, we're still OK
func (s *ConnSuite) TestStartRetriesConnect(c *C) {
	timeouts := []config.ConfigTimeDuration{config.ConfigTimeDuration{0}}
	cfg := Config{ConnectTimeouts: timeouts}
	tb := testingbus.New(condition.Fail2Work(2), condition.Work(true), uint32(networkmanager.Connecting))
	cs := connectedState{config: cfg, log: nullog, bus: tb}

	c.Check(cs.Start(), Equals, networkmanager.Connecting)
	c.Check(cs.connAttempts, Equals, uint32(3)) // 1 more than the Fail2Work
}

// when the calls to NetworkManager fail for a bit, we're still OK
func (s *ConnSuite) TestStartRetriesCall(c *C) {
	cfg := Config{}
	tb := testingbus.New(condition.Work(true), condition.Fail2Work(5), uint32(networkmanager.Connecting))
	cs := connectedState{config: cfg, log: nullog, bus: tb}

	c.Check(cs.Start(), Equals, networkmanager.Connecting)

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
	cfg := Config{}
	tb := testingbus.New(condition.Work(true), nmcond,
		uint32(networkmanager.Connecting),
		uint32(networkmanager.ConnectedGlobal))
	cs := connectedState{config: cfg, log: nullog, bus: tb}

	c.Check(cs.Start(), Equals, networkmanager.Connecting)
	c.Check(cs.connAttempts, Equals, uint32(2))
	c.Check(<-cs.C, Equals, networkmanager.Connecting)
	c.Check(<-cs.C, Equals, networkmanager.ConnectedGlobal)
}

/*
   tests for connectedStateStep()
*/

func (s *ConnSuite) TestSteps(c *C) {
	webget_works := func(ch chan<- bool) { ch <- true }
	webget_fails := func(ch chan<- bool) { ch <- false }

	cfg := Config{
		RecheckTimeout: config.ConfigTimeDuration{10 * time.Millisecond},
	}
	ch := make(chan networkmanager.State, 10)
	cs := &connectedState{
		config:   cfg,
		C:        ch,
		timer:    time.NewTimer(time.Second),
		log:      nullog,
		webget:   webget_works,
		lastSent: false,
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
	cs.webget = webget_fails
	ch <- networkmanager.ConnectedGlobal
	f, e = cs.connectedStateStep()
	c.Check(e, IsNil)
	c.Check(f, Equals, false) // first false is from assuming a Connected signals trouble

	// the next call to Step will time out
	_c := make(chan bool, 1)
	_t := time.NewTimer(10 * time.Millisecond)

	go func() {
		f, e := cs.connectedStateStep()
		c.Check(e, IsNil)
		_c <- f
	}()

	select {
	case <-_c:
		c.Fatal("test failed to timeout")
	case <-_t.C:
	}

	// put it back together again
	cs.webget = webget_works
	// now an recheckTimeout later, we'll get true
	c.Check(<-_c, Equals, true)

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

// Todo: get rid of duplication between this and webchecker_test
const (
	staticText = "something ipsum dolor something"
	staticHash = "6155f83b471583f47c99998a472a178f"
)

// mkHandler makes an http.HandlerFunc that returns the provided text
// for whatever request it's given.
func mkHandler(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		w.Write([]byte(text))
		w.(http.Flusher).Flush()
	}
}

// :oboT

// yes, this is an integration test
func (s *ConnSuite) TestRun(c *C) {
	ts := httptest.NewServer(mkHandler(staticText))
	defer ts.Close()

	cfg := Config{
		ConnectivityCheckURL: ts.URL,
		ConnectivityCheckMD5: staticHash,
		RecheckTimeout:       config.ConfigTimeDuration{time.Second},
	}

	busType := testingbus.New(condition.Work(true), condition.Work(true),
		uint32(networkmanager.ConnectedGlobal),
		uint32(networkmanager.ConnectedGlobal),
		uint32(networkmanager.Disconnected),
	)

	out := make(chan bool)
	dt := time.Second / 10
	timer := time.NewTimer(dt)
	go ConnectedState(busType, cfg, nullog, out)
	var v bool
	expecteds := []bool{
		false, // first state is always false
		true,  // then it should be true as per ConnectedGlobal above
		false, // then, false (upon receiving the next ConnectedGlobal)
		true,  // then it should be true (webcheck passed)
		false, // then it should be false (Disconnected)
		false, // then it should be false again because it's restarted
	}

	for i, expected := range expecteds {
		timer.Reset(dt)
		select {
		case v = <-out:
			break
		case <-timer.C:
			c.Fatalf("Timed out before getting value (#%d)", i+1)
		}

		c.Check(v, Equals, expected)
	}
}
