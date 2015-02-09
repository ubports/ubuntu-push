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

package util

import (
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/bus"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/testing/condition"
	"testing"
	"time"
)

// hook up gocheck
func TestRedialer(t *testing.T) { TestingT(t) }

type RedialerSuite struct {
	timeouts []time.Duration
}

var _ = Suite(&RedialerSuite{})

func (s *RedialerSuite) SetUpSuite(c *C) {
	s.timeouts = SwapTimeouts([]time.Duration{0, 0})
}

func (s *RedialerSuite) TearDownSuite(c *C) {
	SwapTimeouts(s.timeouts)
	s.timeouts = nil
}

// Redial() tests

func (s *RedialerSuite) TestWorks(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Fail2Work(3), nil)
	ar := NewAutoRedialer(endp)
	//	c.Check(ar.(*autoRedialer).stop, NotNil)
	c.Check(ar.Redial(), Equals, uint32(4))
	// and on success, the stopper goes away
	//	c.Check(ar.(*autoRedialer).stop, IsNil)
}

func (s *RedialerSuite) TestRetryNil(c *C) {
	var ar *autoRedialer
	c.Check(ar.Redial, Not(PanicMatches), ".* nil pointer dereference")
}

func (s *RedialerSuite) TestRetryTwice(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Work(true), nil)
	ar := NewAutoRedialer(endp)
	c.Check(ar.Redial(), Equals, uint32(1))
	c.Check(ar.Redial(), Equals, uint32(0))
}

type JitteringEndpoint struct {
	bus.Endpoint
	jittered int
}

func (j *JitteringEndpoint) Jitter(time.Duration) time.Duration {
	j.jittered++
	return 0
}

func (s *RedialerSuite) TestJitterWorks(c *C) {
	endp := &JitteringEndpoint{
		testibus.NewTestingEndpoint(condition.Fail2Work(3), nil),
		0,
	}
	ar := NewAutoRedialer(endp)
	c.Check(ar.Redial(), Equals, uint32(4))
	c.Check(endp.jittered, Equals, 3)
}

// Stop() tests

func (s *RedialerSuite) TestStopWorksOnNil(c *C) {
	// as a convenience, Stop() should succeed on nil
	// (a nil retrier certainly isn't retrying!)
	var ar *autoRedialer
	c.Check(ar, IsNil)
	ar.Stop() // nothing happens
}

func (s *RedialerSuite) TestStopStops(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Work(false), nil)
	countCh := make(chan uint32)
	ar := NewAutoRedialer(endp)
	go func() { countCh <- ar.Redial() }()
	ar.Stop()
	select {
	case <-countCh:
		// pass
	case <-time.After(20 * time.Millisecond):
		c.Fatal("timed out waiting for redial")
	}
	// on Stop(), the redialer is Stopped
	c.Check(ar.(*autoRedialer).state(), Equals, Stopped)
	// and the next Stop() doesn't panic nor block
	ar.Stop()
}
