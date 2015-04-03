/*
 Copyright 2013-2015 Canonical Ltd.

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
	"testing"

	"launchpad.net/go-dbus/v1"
	. "launchpad.net/gocheck"

	testingbus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type NMSuite struct {
	log logger.Logger
}

var _ = Suite(&NMSuite{})

func (s *NMSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
}

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
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(true)), s.log)
	c.Check(nm, NotNil)
}

// GetState returns the right state when everything works
func (s *NMSuite) TestGetState(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(true), uint32(ConnectedGlobal)), s.log)
	state := nm.GetState()
	c.Check(state, Equals, ConnectedGlobal)
}

// GetState returns the right state when dbus fails
func (s *NMSuite) TestGetStateFail(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(false), uint32(ConnectedGlobal)), s.log)
	state := nm.GetState()
	c.Check(state, Equals, Unknown)
}

// GetState returns the right state when dbus works but delivers rubbish values
func (s *NMSuite) TestGetStateRubbishValues(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(true), "Unknown"), s.log)
	state := nm.GetState()
	c.Check(state, Equals, Unknown)
}

// GetState returns the right state when dbus works but delivers a rubbish structure
func (s *NMSuite) TestGetStateRubbishStructure(c *C) {
	nm := New(testingbus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{}), s.log)
	state := nm.GetState()
	c.Check(state, Equals, Unknown)
}

// WatchState sends a stream of States over the channel
func (s *NMSuite) TestWatchState(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true), uint32(Unknown), uint32(Asleep), uint32(ConnectedGlobal))
	nm := New(tc, s.log)
	ch, w, err := nm.WatchState()
	c.Assert(err, IsNil)
	defer w.Cancel()
	l := []State{<-ch, <-ch, <-ch}
	c.Check(l, DeepEquals, []State{Unknown, Asleep, ConnectedGlobal})
}

// WatchState returns on error if the dbus call fails
func (s *NMSuite) TestWatchStateFails(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(false)), s.log)
	_, _, err := nm.WatchState()
	c.Check(err, NotNil)
}

// WatchState calls close on its channel when the watch bails
func (s *NMSuite) TestWatchStateClosesOnWatchBail(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true))
	nm := New(tc, s.log)
	ch, w, err := nm.WatchState()
	c.Assert(err, IsNil)
	defer w.Cancel()
	_, ok := <-ch
	c.Check(ok, Equals, false)
}

// WatchState survives rubbish values
func (s *NMSuite) TestWatchStateSurvivesRubbishValues(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true), "a")
	nm := New(tc, s.log)
	ch, w, err := nm.WatchState()
	c.Assert(err, IsNil)
	defer w.Cancel()
	_, ok := <-ch
	c.Check(ok, Equals, false)
}

// GetPrimaryConnection returns the right state when everything works
func (s *NMSuite) TestGetPrimaryConnection(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(true), dbus.ObjectPath("/a/1")), s.log)
	con := nm.GetPrimaryConnection()
	c.Check(con, Equals, "/a/1")
}

// GetPrimaryConnection returns the right state when dbus fails
func (s *NMSuite) TestGetPrimaryConnectionFail(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(false)), s.log)
	con := nm.GetPrimaryConnection()
	c.Check(con, Equals, "")
}

// GetPrimaryConnection returns the right state when dbus works but delivers rubbish values
func (s *NMSuite) TestGetPrimaryConnectionRubbishValues(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(true), "broken"), s.log)
	con := nm.GetPrimaryConnection()
	c.Check(con, Equals, "")
}

// GetPrimaryConnection returns the right state when dbus works but delivers a rubbish structure
func (s *NMSuite) TestGetPrimaryConnectionRubbishStructure(c *C) {
	nm := New(testingbus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{}), s.log)
	con := nm.GetPrimaryConnection()
	c.Check(con, Equals, "")
}

func mkPriConMap(priCon string) map[string]dbus.Variant {
	m := make(map[string]dbus.Variant)
	m["PrimaryConnection"] = dbus.Variant{dbus.ObjectPath(priCon)}
	return m
}

// WatchPrimaryConnection sends a stream of Connections over the channel
func (s *NMSuite) TestWatchPrimaryConnection(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true),
		mkPriConMap("/a/1"),
		mkPriConMap("/b/2"),
		mkPriConMap("/c/3"))
	nm := New(tc, s.log)
	ch, w, err := nm.WatchPrimaryConnection()
	c.Assert(err, IsNil)
	defer w.Cancel()
	l := []string{<-ch, <-ch, <-ch}
	c.Check(l, DeepEquals, []string{"/a/1", "/b/2", "/c/3"})
}

// WatchPrimaryConnection returns on error if the dbus call fails
func (s *NMSuite) TestWatchPrimaryConnectionFails(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(false)), s.log)
	_, _, err := nm.WatchPrimaryConnection()
	c.Check(err, NotNil)
}

// WatchPrimaryConnection calls close on its channel when the watch bails
func (s *NMSuite) TestWatchPrimaryConnectionClosesOnWatchBail(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true))
	nm := New(tc, s.log)
	ch, w, err := nm.WatchPrimaryConnection()
	c.Assert(err, IsNil)
	defer w.Cancel()
	_, ok := <-ch
	c.Check(ok, Equals, false)
}

// WatchPrimaryConnection survives rubbish values
func (s *NMSuite) TestWatchPrimaryConnectionSurvivesRubbishValues(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true), "a")
	nm := New(tc, s.log)
	ch, w, err := nm.WatchPrimaryConnection()
	c.Assert(err, IsNil)
	defer w.Cancel()
	_, ok := <-ch
	c.Check(ok, Equals, false)
}

// WatchPrimaryConnection ignores non-PrimaryConnection PropertyChanged
func (s *NMSuite) TestWatchPrimaryConnectionIgnoresIrrelephant(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true),
		map[string]dbus.Variant{"foo": dbus.Variant{}},
		map[string]dbus.Variant{"PrimaryConnection": dbus.Variant{dbus.ObjectPath("42")}},
	)
	nm := New(tc, s.log)
	ch, w, err := nm.WatchPrimaryConnection()
	c.Assert(err, IsNil)
	defer w.Cancel()
	v, ok := <-ch
	c.Check(ok, Equals, true)
	c.Check(v, Equals, "42")
}

// WatchPrimaryConnection ignores rubbish PrimaryConnections
func (s *NMSuite) TestWatchPrimaryConnectionIgnoresRubbishValues(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true),
		map[string]dbus.Variant{"PrimaryConnection": dbus.Variant{-12}},
		map[string]dbus.Variant{"PrimaryConnection": dbus.Variant{dbus.ObjectPath("42")}},
	)
	nm := New(tc, s.log)
	ch, w, err := nm.WatchPrimaryConnection()
	c.Assert(err, IsNil)
	defer w.Cancel()
	v, ok := <-ch
	c.Check(ok, Equals, true)
	c.Check(v, Equals, "42")
}

// GetWirelessEnabled returns the right state when everything works
func (s *NMSuite) TestGetWirelessEnabled(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(true), false), s.log)
	en := nm.GetWirelessEnabled()
	c.Check(en, Equals, false)
}

// GetWirelessEnabled returns the right state when dbus fails
func (s *NMSuite) TestGetWirelessEnabledFail(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(false)), s.log)
	en := nm.GetWirelessEnabled()
	c.Check(en, Equals, true)
}

// GetWirelessEnabled returns the right state when dbus works but delivers rubbish values
func (s *NMSuite) TestGetWirelessEnabledRubbishValues(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(true), "broken"), s.log)
	en := nm.GetWirelessEnabled()
	c.Check(en, Equals, true)
}

// GetWirelessEnabled returns the right state when dbus works but delivers a rubbish structure
func (s *NMSuite) TestGetWirelessEnabledRubbishStructure(c *C) {
	nm := New(testingbus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{}), s.log)
	en := nm.GetWirelessEnabled()
	c.Check(en, Equals, true)
}

func mkWirelessEnMap(en bool) map[string]dbus.Variant {
	m := make(map[string]dbus.Variant)
	m["WirelessEnabled"] = dbus.Variant{en}
	return m
}

// WatchWirelessEnabled sends a stream of wireless enabled states over the channel
func (s *NMSuite) TestWatchWirelessEnabled(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true),
		mkWirelessEnMap(true),
		mkWirelessEnMap(false),
		mkWirelessEnMap(true),
	)
	nm := New(tc, s.log)
	ch, w, err := nm.WatchWirelessEnabled()
	c.Assert(err, IsNil)
	defer w.Cancel()
	l := []bool{<-ch, <-ch, <-ch}
	c.Check(l, DeepEquals, []bool{true, false, true})
}

// WatchWirelessEnabled returns on error if the dbus call fails
func (s *NMSuite) TestWatchWirelessEnabledFails(c *C) {
	nm := New(testingbus.NewTestingEndpoint(nil, condition.Work(false)), s.log)
	_, _, err := nm.WatchWirelessEnabled()
	c.Check(err, NotNil)
}

// WatchWirelessEnabled calls close on its channel when the watch bails
func (s *NMSuite) TestWatchWirelessEnabledClosesOnWatchBail(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true))
	nm := New(tc, s.log)
	ch, w, err := nm.WatchWirelessEnabled()
	c.Assert(err, IsNil)
	defer w.Cancel()
	_, ok := <-ch
	c.Check(ok, Equals, false)
}

// WatchWirelessEnabled survives rubbish values
func (s *NMSuite) TestWatchWirelessEnabledSurvivesRubbishValues(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true), "gorp")
	nm := New(tc, s.log)
	ch, w, err := nm.WatchWirelessEnabled()
	c.Assert(err, IsNil)
	defer w.Cancel()
	_, ok := <-ch
	c.Check(ok, Equals, false)
}

// WatchWirelessEnabled ignores non-WirelessEnabled PropertyChanged
func (s *NMSuite) TestWatchWirelessEnabledIgnoresIrrelephant(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true),
		map[string]dbus.Variant{"foo": dbus.Variant{}},
		map[string]dbus.Variant{"WirelessEnabled": dbus.Variant{true}},
	)
	nm := New(tc, s.log)
	ch, w, err := nm.WatchWirelessEnabled()
	c.Assert(err, IsNil)
	defer w.Cancel()
	v, ok := <-ch
	c.Check(ok, Equals, true)
	c.Check(v, Equals, true)
}

// WatchWirelessEnabled ignores rubbish WirelessEnabled
func (s *NMSuite) TestWatchWirelessEnabledIgnoresRubbishValues(c *C) {
	tc := testingbus.NewTestingEndpoint(nil, condition.Work(true),
		map[string]dbus.Variant{"WirelessEnabled": dbus.Variant{-12}},
		map[string]dbus.Variant{"WirelessEnabled": dbus.Variant{false}},
	)
	nm := New(tc, s.log)
	ch, w, err := nm.WatchWirelessEnabled()
	c.Assert(err, IsNil)
	defer w.Cancel()
	v, ok := <-ch
	c.Check(ok, Equals, true)
	c.Check(v, Equals, false)
}
