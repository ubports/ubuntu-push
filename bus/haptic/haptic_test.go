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

package haptic

import (
	"testing"

	. "launchpad.net/gocheck"

	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/click"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	"launchpad.net/ubuntu-push/launch_helper"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

func TestHaptic(t *testing.T) { TestingT(t) }

type hapticSuite struct {
	log *helpers.TestLogger
	app *click.AppId
}

var _ = Suite(&hapticSuite{})

func (hs *hapticSuite) SetUpTest(c *C) {
	hs.log = helpers.NewTestLogger(c, "debug")
	hs.app = clickhelp.MustParseAppId("com.example.test_test-app_0")
}

// checks that Present() actually calls VibratePattern
func (hs *hapticSuite) TestPresentPresents(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))

	ec := New(endp, hs.log)
	notif := launch_helper.Notification{Vibrate: &launch_helper.Vibration{Pattern: []uint32{200, 100}, Repeat: 2}}
	c.Check(ec.Present(hs.app, "nid", &notif), Equals, true)
	callArgs := testibus.GetCallArgs(endp)
	c.Assert(callArgs, HasLen, 1)
	c.Check(callArgs[0].Member, Equals, "VibratePattern")
	c.Check(callArgs[0].Args, DeepEquals, []interface{}{[]uint32{200, 100}, uint32(2)})
}

// check that Present() defaults Repeat to 1
func (hs *hapticSuite) TestPresentDefaultsRepeatTo1(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))

	ec := New(endp, hs.log)
	// note: no Repeat:
	notif := launch_helper.Notification{Vibrate: &launch_helper.Vibration{Pattern: []uint32{200, 100}}}
	c.Check(ec.Present(hs.app, "nid", &notif), Equals, true)
	callArgs := testibus.GetCallArgs(endp)
	c.Assert(callArgs, HasLen, 1)
	c.Check(callArgs[0].Member, Equals, "VibratePattern")
	// note: Repeat of 1:
	c.Check(callArgs[0].Args, DeepEquals, []interface{}{[]uint32{200, 100}, uint32(1)})
}

// check that Present() doesn't call VibratePattern if things are not right
func (hs *hapticSuite) TestSkipIfMissing(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))

	ec := New(endp, hs.log)
	// no Vibration in the notificaton
	c.Check(ec.Present(hs.app, "", &launch_helper.Notification{}), Equals, false)
	// empty Vibration
	c.Check(ec.Present(hs.app, "", &launch_helper.Notification{Vibrate: &launch_helper.Vibration{}}), Equals, false)
}

// check that Present() panics if the notification is nil
func (hs *hapticSuite) TestPanicsIfNil(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))

	ec := New(endp, hs.log)
	// no notification at all
	c.Check(func() { ec.Present(hs.app, "", nil) }, Panics, `please check notification is not nil before calling present`)
}
