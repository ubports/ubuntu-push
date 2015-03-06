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

package accounts

import (
	"errors"
	"testing"

	"launchpad.net/go-dbus/v1"
	. "launchpad.net/gocheck"

	testibus "launchpad.net/ubuntu-push/bus/testing"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

// hook up gocheck
func TestAcc(t *testing.T) { TestingT(t) }

type AccSuite struct {
	log *helpers.TestLogger
}

var _ = Suite(&AccSuite{})

type TestCancellable struct {
	canceled bool
	err      error
}

func (t *TestCancellable) Cancel() error {
	t.canceled = true
	return t.err
}

func (s *AccSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
}

func (s *AccSuite) TestBusAddressPathUidLoaded(c *C) {
	c.Check(BusAddress.Path, Matches, `.*\d+`)
}

func (s *AccSuite) TestCancelCancelsCancellable(c *C) {
	err := errors.New("cancel error")
	t := &TestCancellable{err: err}
	a := New(nil, s.log).(*accounts)
	a.cancellable = t

	c.Check(a.Cancel(), Equals, err)
	c.Check(t.canceled, Equals, true)
}

func (s *AccSuite) TestStartReportsWatchError(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	a := New(endp, s.log).(*accounts)
	c.Assert(a, NotNil)

	err := a.Start()
	c.Check(err, NotNil)
}

func (s *AccSuite) TestStartSetsCancellable(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), true)
	a := New(endp, s.log).(*accounts)
	c.Assert(a, NotNil)

	c.Check(a.cancellable, IsNil)
	err := a.Start()
	c.Check(err, IsNil)
	c.Check(a.cancellable, NotNil)
	a.Cancel()
}

func (s *AccSuite) TestStartPanicsIfCalledTwice(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), true, true)
	a := New(endp, s.log).(*accounts)
	c.Assert(a, NotNil)

	c.Check(a.cancellable, IsNil)
	err := a.Start()
	c.Check(err, IsNil)
	c.Check(func() { a.startWatch() }, PanicMatches, `.* twice\?`)
	a.Cancel()
}

func (s *AccSuite) TestUpdateCallsUpdaters(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true),
		map[string]dbus.Variant{"x": dbus.Variant{"hello"}})
	a := New(endp, s.log).(*accounts)
	c.Assert(a, NotNil)
	var x dbus.Variant
	a.updaters = map[string]func(dbus.Variant){
		"x": func(v dbus.Variant) { x = v },
	}
	a.update()

	c.Check(x.Value, Equals, "hello")
}

func (s *AccSuite) TestUpdateSilentModeBails(c *C) {
	a := New(nil, s.log).(*accounts)
	a.updateSilentMode(dbus.Variant{"rubbish"})
	c.Check(s.log.Captured(), Matches, `(?ms)ERROR SilentMode needed a bool.`)
}

func (s *AccSuite) TestUpdateSilentModeWorks(c *C) {
	a := New(nil, s.log).(*accounts)
	c.Check(a.silent, Equals, false)
	a.updateSilentMode(dbus.Variant{true})
	c.Check(a.silent, Equals, true)
}

func (s *AccSuite) TestUpdateVibrateBails(c *C) {
	a := New(nil, s.log).(*accounts)
	a.updateVibrate(dbus.Variant{"rubbish"})
	c.Check(s.log.Captured(), Matches, `(?ms)ERROR IncomingMessageVibrate needed a bool.`)
}

func (s *AccSuite) TestUpdateVibrateWorks(c *C) {
	a := New(nil, s.log).(*accounts)
	c.Check(a.vibrate, Equals, false)
	a.updateVibrate(dbus.Variant{true})
	c.Check(a.vibrate, Equals, true)
}

func (s *AccSuite) TestUpdateVibrateSilentModeBails(c *C) {
	a := New(nil, s.log).(*accounts)
	a.updateVibrateSilentMode(dbus.Variant{"rubbish"})
	c.Check(s.log.Captured(), Matches, `(?ms)ERROR IncomingMessageVibrateSilentMode needed a bool.`)
}

func (s *AccSuite) TestUpdateVibrateSilentModeWorks(c *C) {
	a := New(nil, s.log).(*accounts)
	c.Check(a.vibrateSilentMode, Equals, false)
	a.updateVibrateSilentMode(dbus.Variant{true})
	c.Check(a.vibrateSilentMode, Equals, true)
}

func (s *AccSuite) TestUpdateMessageSoundBails(c *C) {
	a := New(nil, s.log).(*accounts)
	a.updateMessageSound(dbus.Variant{42})
	c.Check(s.log.Captured(), Matches, `(?ms)ERROR IncomingMessageSound needed a string.`)
}

func (s *AccSuite) TestUpdateMessageSoundWorks(c *C) {
	a := New(nil, s.log).(*accounts)
	c.Check(a.messageSound, Equals, "")
	a.updateMessageSound(dbus.Variant{"xyzzy"})
	c.Check(a.messageSound, Equals, "xyzzy")
}

func (s *AccSuite) TestUpdateMessageSoundPrunesXDG(c *C) {
	a := New(nil, s.log).(*accounts)
	a.updateMessageSound(dbus.Variant{"/usr/share/xyzzy"})
	c.Check(a.messageSound, Equals, "xyzzy")
}

func (s *AccSuite) TestPropsHandler(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))

	// testing a series of bad args for propsHandler:  none,
	New(endp, s.log).(*accounts).propsHandler()
	c.Check(s.log.Captured(), Matches, `(?ms).*ERROR PropertiesChanged delivered 0 things.*`)
	s.log.ResetCapture()

	// bad type for all,
	New(endp, s.log).(*accounts).propsHandler(nil, nil, nil)
	c.Check(s.log.Captured(), Matches, `(?ms).*ERROR PropertiesChanged 1st param not a string.*`)
	s.log.ResetCapture()

	// wrong interface,
	New(endp, s.log).(*accounts).propsHandler("xyzzy", nil, nil)
	c.Check(s.log.Captured(), Matches, `(?ms).*DEBUG PropertiesChanged for "xyzzy", ignoring\..*`)
	s.log.ResetCapture()

	// bad type for 2nd and 3rd,
	New(endp, s.log).(*accounts).propsHandler(accountsSoundIface, nil, nil)
	c.Check(s.log.Captured(), Matches, `(?ms).*ERROR PropertiesChanged 2nd param not a map.*`)
	s.log.ResetCapture()

	// not-seen-in-the-wild 'changed' argument (first non-error outcome),
	New(endp, s.log).(*accounts).propsHandler(accountsSoundIface, map[interface{}]interface{}{"x": "y"}, nil)
	// tracking the update() via the GetAll call it generates (which will fail because of the testibus of Work(false) above)
	c.Check(s.log.Captured(), Matches, `(?ms).*INFO PropertiesChanged provided 'changed'.*ERROR when calling GetAll.*`)
	s.log.ResetCapture()

	// bad type for 3rd (with empty 2nd),
	New(endp, s.log).(*accounts).propsHandler(accountsSoundIface, map[interface{}]interface{}{}, nil)
	c.Check(s.log.Captured(), Matches, `(?ms).*ERROR PropertiesChanged 3rd param not a list of properties.*`)
	s.log.ResetCapture()

	// bad type for elements of 3rd,
	New(endp, s.log).(*accounts).propsHandler(accountsSoundIface, map[interface{}]interface{}{}, []interface{}{42})
	c.Check(s.log.Captured(), Matches, `(?ms).*ERROR PropertiesChanged 3rd param's only entry not a string.*`)
	s.log.ResetCapture()

	// empty 3rd (not an error; hard to test "do ),
	New(endp, s.log).(*accounts).propsHandler(accountsSoundIface, map[interface{}]interface{}{}, []interface{}{})
	c.Check(s.log.Captured(), Matches, `(?ms).*DEBUG PropertiesChanged 3rd param is empty.*`)
	s.log.ResetCapture()

	// more than one 2rd (also not an error; again looking at the GetAll failure to confirm update() got called),
	New(endp, s.log).(*accounts).propsHandler(accountsSoundIface, map[interface{}]interface{}{}, []interface{}{"hi", "there"})
	c.Check(s.log.Captured(), Matches, `(?ms).*INFO.* reverting to full update.*ERROR when calling GetAll.*`)
	s.log.ResetCapture()

	// bus trouble for a single entry in the 3rd,
	New(endp, s.log).(*accounts).propsHandler(accountsSoundIface, map[interface{}]interface{}{}, []interface{}{"SilentMode"})
	c.Check(s.log.Captured(), Matches, `(?ms).*ERROR when calling Get for SilentMode.*`)
	s.log.ResetCapture()

	// and finally, the common case: a single entry in the 3rd param, that gets updated individually.
	xOuter := dbus.Variant{"x"}
	a := New(testibus.NewTestingEndpoint(nil, condition.Work(true), xOuter), s.log).(*accounts)
	called := false
	a.updaters = map[string]func(dbus.Variant){"xyzzy": func(x dbus.Variant) {
		c.Check(x, Equals, xOuter)
		called = true
	}}
	a.propsHandler(accountsSoundIface, map[interface{}]interface{}{}, []interface{}{"xyzzy"})
	c.Check(called, Equals, true)
}

func (s *AccSuite) TestSilentMode(c *C) {
	a := New(nil, s.log).(*accounts)
	c.Check(a.SilentMode(), Equals, false)
	a.silent = true
	c.Check(a.SilentMode(), Equals, true)
}

func (s *AccSuite) TestVibrate(c *C) {
	a := New(nil, s.log).(*accounts)
	c.Check(a.Vibrate(), Equals, false)
	a.vibrate = true
	c.Check(a.Vibrate(), Equals, true)
	a.silent = true
	c.Check(a.Vibrate(), Equals, false)
	a.vibrateSilentMode = true
	c.Check(a.Vibrate(), Equals, true)
	a.vibrate = false
	c.Check(a.Vibrate(), Equals, true)
}

func (s *AccSuite) TestMessageSoundFile(c *C) {
	a := New(nil, s.log).(*accounts)
	c.Check(a.MessageSoundFile(), Equals, "")
	a.messageSound = "xyzzy"
	c.Check(a.MessageSoundFile(), Equals, "xyzzy")
}

func (s *AccSuite) TestString(c *C) {
	a := New(nil, s.log).(*accounts)
	a.vibrate = true
	a.messageSound = "x"
	c.Check(a.String(), Equals, `&accounts{silent: false, vibrate: true, vibratesilent: false, messageSound: "x"}`)
}
