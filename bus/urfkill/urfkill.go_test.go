/*
 Copyright 2015 Canonical Ltd.

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

package urfkill

import (
	"testing"

	//"launchpad.net/go-dbus/v1"
	. "launchpad.net/gocheck"

	testingbus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type URSuite struct {
	log logger.Logger
}

var _ = Suite(&URSuite{})

func (s *URSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
}

func (s *URSuite) TestNew(c *C) {
	ur := New(testingbus.NewTestingEndpoint(nil, condition.Work(true)), s.log)
	c.Check(ur, NotNil)
}

// IsFlightMode returns the right state when everything works
func (s *URSuite) TestIsFlightMode(c *C) {
	endp := testingbus.NewTestingEndpoint(nil, condition.Work(true), true)
	ur := New(endp, s.log)
	state := ur.IsFlightMode()
	c.Check(state, Equals, true)
	callArgs := testingbus.GetCallArgs(endp)
	c.Assert(callArgs, HasLen, 1)
	c.Assert(callArgs[0].Member, Equals, "IsFlightMode")
	c.Assert(callArgs[0].Args, HasLen, 0)
}

// IsFlightMode returns the right state when dbus fails
func (s *URSuite) TestIsFlightModeFail(c *C) {
	ur := New(testingbus.NewTestingEndpoint(nil, condition.Work(false)), s.log)
	state := ur.IsFlightMode()
	c.Check(state, Equals, false)
}

// IsFlightMode returns the right state when dbus works but delivers
// rubbish values
func (s *URSuite) TestIsFlightModeRubbishValues(c *C) {
	ur := New(testingbus.NewTestingEndpoint(nil, condition.Work(true), "broken"), s.log)
	state := ur.IsFlightMode()
	c.Check(state, Equals, false)
}

// IsFlightMode returns the right state when dbus works but delivers a rubbish structure
func (s *URSuite) TestIsFlightModeRubbishStructure(c *C) {
	ur := New(testingbus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{}), s.log)
	state := ur.IsFlightMode()
	c.Check(state, Equals, false)
}

// WatchFightMode sends a stream of states over the channel
func (s *URSuite) TestWatchFlightMode(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true), false, true, false)
	ur := New(tc, s.log)
	ch, w, err := ur.WatchFlightMode()
	c.Assert(err, IsNil)
	defer w.Cancel()
	l := []bool{<-ch, <-ch, <-ch}
	c.Check(l, DeepEquals, []bool{false, true, false})
}

// WatchFlightMode returns on error if the dbus call fails
func (s *URSuite) TestWatchFlightModeFails(c *C) {
	ur := New(testingbus.NewTestingEndpoint(nil, condition.Work(false)), s.log)
	_, _, err := ur.WatchFlightMode()
	c.Check(err, NotNil)
}

// WatchFlightMode calls close on its channel when the watch bails
func (s *URSuite) TestWatchFlightModeClosesOnWatchBail(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true))
	ur := New(tc, s.log)
	ch, w, err := ur.WatchFlightMode()
	c.Assert(err, IsNil)
	defer w.Cancel()
	_, ok := <-ch
	c.Check(ok, Equals, false)
}

// WatchFlightMode survives rubbish values
func (s *URSuite) TestWatchFlightModeSurvivesRubbishValues(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true), "gorp")
	ur := New(tc, s.log)
	ch, w, err := ur.WatchFlightMode()
	c.Assert(err, IsNil)
	defer w.Cancel()
	_, ok := <-ch
	c.Check(ok, Equals, false)
}
