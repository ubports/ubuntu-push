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

package networkmanager

import (
	"io/ioutil"
	. "launchpad.net/gocheck"
	testingbus "launchpad.net/ubuntu-push/bus/connection/testing"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/testing/condition"
	"testing"
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type NMSuite struct{}

var _ = Suite(&NMSuite{})

var nullog = logger.NewSimpleLogger(ioutil.Discard, "error")

// TestNames checks that networkmanager.State objects serialize
// correctly, to a point.
func (s *NMSuite) TestNames(c *C) {
	var i State
	for i = 0; i < _max_state; i += 10 {
		c.Check(names[i], Equals, i.String())
	}
	i = _max_state
	c.Check(i.String(), Equals, "Unknown")
}

// TestNew doesn't test much at all. If this fails, all is wrong in the world.
func (s *NMSuite) TestNew(c *C) {
	nm := New(testingbus.New(condition.Work(true)), nullog)
	c.Check(nm, NotNil)
}

// GetState returns the right state when everything works
func (s *NMSuite) TestGetState(c *C) {
	nm := New(testingbus.New(condition.Work(true), uint32(ConnectedGlobal)), nullog)
	state := nm.GetState()
	c.Check(state, Equals, ConnectedGlobal)
}

// GetState returns the right state when dbus fails
func (s *NMSuite) TestGetStateFail(c *C) {
	nm := New(testingbus.New(condition.Work(false), uint32(ConnectedGlobal)), nullog)
	state := nm.GetState()
	c.Check(state, Equals, Unknown)
}

// GetState returns the right state when dbus works but delivers rubbish
func (s *NMSuite) TestGetStateRubbish(c *C) {
	nm := New(testingbus.New(condition.Work(false), 42), nullog)
	state := nm.GetState()
	c.Check(state, Equals, Unknown)
}

// WatchState sends a stream of States over the channel
func (s *NMSuite) TestWatchState(c *C) {
	tc := testingbus.New(condition.Work(true), uint32(Unknown), uint32(Asleep), uint32(ConnectedGlobal))
	nm := New(tc, nullog)
	ch, err := nm.WatchState()
	c.Check(err, IsNil)
	l := []State{<-ch, <-ch, <-ch}
	c.Check(l, DeepEquals, []State{Unknown, Asleep, ConnectedGlobal})
}

// WatchState returns on error if the dbus call fails
func (s *NMSuite) TestWatchStateFails(c *C) {
	nm := New(testingbus.New(condition.Work(false)), nullog)
	_, err := nm.WatchState()
	c.Check(err, NotNil)
}
