/*
 Copyright 2014 Canonical Ltd.

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

package powerd

import (
	"testing"
	"time"

	. "launchpad.net/gocheck"

	testibus "launchpad.net/ubuntu-push/bus/testing"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

// hook up gocheck
func TestPowerd(t *testing.T) { TestingT(t) }

type PdSuite struct {
	log *helpers.TestLogger
}

var _ = Suite(&PdSuite{})

func (s *PdSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
}

func (s *PdSuite) TestRequestWakeupWorks(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), "cookie")
	pd := New(endp, s.log)
	t := time.Now().Add(5 * time.Minute)
	ck, err := pd.RequestWakeup("name", t)
	c.Assert(err, IsNil)
	c.Check(ck, Equals, "cookie")
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "requestWakeup")
	c.Check(args[0].Args, DeepEquals, []interface{}{"name", uint64(t.Unix())})
}

func (s *PdSuite) TestRequestWakeupUnconfigured(c *C) {
	_, err := new(powerd).RequestWakeup("name", time.Now())
	c.Assert(err, Equals, ErrUnconfigured)
}

func (s *PdSuite) TestRequestWakeupFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	pd := New(endp, s.log)
	t := time.Now().Add(5 * time.Minute)
	_, err := pd.RequestWakeup("name", t)
	c.Assert(err, NotNil)
}

func (s *PdSuite) TestClearWakeupWorks(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))
	pd := New(endp, s.log)
	err := pd.ClearWakeup("cookie")
	c.Assert(err, IsNil)
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "clearWakeup")
	c.Check(args[0].Args, DeepEquals, []interface{}{"cookie"})
}

func (s *PdSuite) TestClearWakeupUnconfigured(c *C) {
	err := new(powerd).ClearWakeup("cookie")
	c.Assert(err, Equals, ErrUnconfigured)
}

func (s *PdSuite) TestClearWakeupFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	pd := New(endp, s.log)
	err := pd.ClearWakeup("cookie")
	c.Assert(err, NotNil)
}

func (s *PdSuite) TestRequestWakelockWorks(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), "cookie")
	pd := New(endp, s.log)
	ck, err := pd.RequestWakelock("name")
	c.Assert(err, IsNil)
	c.Check(ck, Equals, "cookie")
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 1)
	// wakelocks are documented on https://wiki.ubuntu.com/powerd#API
	c.Check(args[0].Member, Equals, "requestSysState")
	c.Check(args[0].Args, DeepEquals, []interface{}{"name", int32(1)})
}

func (s *PdSuite) TestRequestWakelockUnconfigured(c *C) {
	_, err := new(powerd).RequestWakelock("name")
	c.Assert(err, Equals, ErrUnconfigured)
}

func (s *PdSuite) TestRequestWakelockFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	pd := New(endp, s.log)
	_, err := pd.RequestWakelock("name")
	c.Assert(err, NotNil)
}

func (s *PdSuite) TestClearWakelockWorks(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))
	pd := New(endp, s.log)
	err := pd.ClearWakelock("cookie")
	c.Assert(err, IsNil)
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "clearSysState")
	c.Check(args[0].Args, DeepEquals, []interface{}{"cookie"})
}

func (s *PdSuite) TestClearWakelockUnconfigured(c *C) {
	c.Check(new(powerd).ClearWakelock("cookie"), NotNil)
}

func (s *PdSuite) TestClearWakelockFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	pd := New(endp, s.log)
	err := pd.ClearWakelock("cookie")
	c.Assert(err, NotNil)
}

func (s *PdSuite) TestWatchWakeupsWorks(c *C) {
	endp := testibus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{})
	pd := New(endp, s.log)
	ch, err := pd.WatchWakeups()
	c.Assert(err, IsNil)
	select {
	case b := <-ch:
		c.Check(b, Equals, true)
	case <-time.After(100 * time.Millisecond):
		c.Error("timeout waiting for bool")
	}
	select {
	case b := <-ch:
		c.Check(b, Equals, false)
	case <-time.After(100 * time.Millisecond):
		c.Error("timeout waiting for close")
	}
}

func (s *PdSuite) TestWatchWakeupsUnconfigured(c *C) {
	_, err := new(powerd).WatchWakeups()
	c.Check(err, Equals, ErrUnconfigured)
}
