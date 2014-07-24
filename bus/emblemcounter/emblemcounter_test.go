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

package emblemcounter

import (
	"testing"

	"launchpad.net/go-dbus/v1"
	. "launchpad.net/gocheck"

	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/click"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/nih"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

func TestEmblemCounter(t *testing.T) { TestingT(t) }

type ecSuite struct {
	log *helpers.TestLogger
	app *click.AppId
}

var _ = Suite(&ecSuite{})

func (ecs *ecSuite) SetUpTest(c *C) {
	ecs.log = helpers.NewTestLogger(c, "debug")
	ecs.app = clickhelp.MustParseAppId("com.example.test_test-app_0")
}

// checks that SetCounter() actually calls SetProperty on the launcher
func (ecs *ecSuite) TestSetCounterSetsTheCounter(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))
	quoted := string(nih.Quote([]byte(ecs.app.Base())))

	ec := New(endp, ecs.log)
	c.Check(ec.SetCounter(ecs.app, "nid", 42, true), Equals, true)
	callArgs := testibus.GetCallArgs(endp)
	c.Assert(callArgs, HasLen, 2)
	c.Check(callArgs[0].Member, Equals, "::SetProperty")
	c.Check(callArgs[1].Member, Equals, "::SetProperty")
	c.Check(callArgs[0].Args, DeepEquals, []interface{}{"count", "/" + quoted, dbus.Variant{Value: int32(42)}})
	c.Check(callArgs[1].Args, DeepEquals, []interface{}{"countVisible", "/" + quoted, dbus.Variant{Value: true}})
}

// checks that Present() actually calls SetProperty on the launcher
func (ecs *ecSuite) TestPresentPresents(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))
	quoted := string(nih.Quote([]byte(ecs.app.Base())))

	ec := New(endp, ecs.log)
	notif := launch_helper.Notification{EmblemCounter: &launch_helper.EmblemCounter{Count: 42, Visible: true}}
	c.Check(ec.Present(ecs.app, "nid", &notif), Equals, true)
	callArgs := testibus.GetCallArgs(endp)
	c.Assert(callArgs, HasLen, 2)
	c.Check(callArgs[0].Member, Equals, "::SetProperty")
	c.Check(callArgs[1].Member, Equals, "::SetProperty")
	c.Check(callArgs[0].Args, DeepEquals, []interface{}{"count", "/" + quoted, dbus.Variant{Value: int32(42)}})
	c.Check(callArgs[1].Args, DeepEquals, []interface{}{"countVisible", "/" + quoted, dbus.Variant{Value: true}})
}

// check that Present() doesn't call SetProperty if no EmblemCounter in the Notification
func (ecs *ecSuite) TestSkipIfMissing(c *C) {
	quoted := string(nih.Quote([]byte(ecs.app.Base())))
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))
	ec := New(endp, ecs.log)

	// nothing happens if no EmblemCounter in Notification
	c.Check(ec.Present(ecs.app, "nid", &launch_helper.Notification{}), Equals, false)
	c.Assert(testibus.GetCallArgs(endp), HasLen, 0)

	// but an empty EmblemCounter is acted on
	c.Check(ec.Present(ecs.app, "nid", &launch_helper.Notification{EmblemCounter: &launch_helper.EmblemCounter{}}), Equals, true)
	callArgs := testibus.GetCallArgs(endp)
	c.Assert(callArgs, HasLen, 2)
	c.Check(callArgs[0].Member, Equals, "::SetProperty")
	c.Check(callArgs[1].Member, Equals, "::SetProperty")
	c.Check(callArgs[0].Args, DeepEquals, []interface{}{"count", "/" + quoted, dbus.Variant{Value: int32(0)}})
	c.Check(callArgs[1].Args, DeepEquals, []interface{}{"countVisible", "/" + quoted, dbus.Variant{Value: false}})
}

// check that Present() panics if the notification is nil
func (ecs *ecSuite) TestPanicsIfNil(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))
	ec := New(endp, ecs.log)

	// nothing happens if no EmblemCounter in Notification
	c.Check(func() { ec.Present(ecs.app, "nid", nil) }, Panics, `please check notification is not nil before calling present`)
}

func buildNotification(tag string, n int32, v bool) *launch_helper.Notification {
	e := &launch_helper.EmblemCounter{Count: n, Visible: v}
	return &launch_helper.Notification{EmblemCounter: e, Tag: tag}
}

// checks that Tags() keeps track of the tags
func (ecs *ecSuite) TestTagsListsTags(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))
	ec := New(endp, ecs.log)
	f := buildNotification

	c.Check(ec.Tags(ecs.app), IsNil)
	c.Assert(ec.Present(ecs.app, "notif1", f("one", 1, true)), Equals, true)
	c.Check(ec.Tags(ecs.app), DeepEquals, []string{"one"})
	// setting one tag clears the previous one
	c.Assert(ec.Present(ecs.app, "notif2", f("two", 1, true)), Equals, true)
	c.Check(ec.Tags(ecs.app), DeepEquals, []string{"two"})
	// but setting a non-visible one clears it
	c.Assert(ec.Present(ecs.app, "notif3", f("three", 1, false)), Equals, true)
	c.Check(ec.Tags(ecs.app), IsNil)
	// (re-adding one...)
	c.Assert(ec.Present(ecs.app, "notif4", f("one", 1, true)), Equals, true)
	c.Check(ec.Tags(ecs.app), DeepEquals, []string{"one"})
	// 0 counts as not visible
	c.Assert(ec.Present(ecs.app, "notif5", f("three", 0, true)), Equals, true)
	c.Check(ec.Tags(ecs.app), IsNil)
	// and an empty notification doesn't count
	c.Assert(ec.Present(ecs.app, "notif6", &launch_helper.Notification{Tag: "xxx"}), Equals, false)
	c.Check(ec.Tags(ecs.app), IsNil)
	// but an empty tag does
	c.Assert(ec.Present(ecs.app, "notif5", f("", 1, true)), Equals, true)
	c.Check(ec.Tags(ecs.app), DeepEquals, []string{""})
}

func (ecs *ecSuite) TestClear(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))
	ec := New(endp, ecs.log)
	f := buildNotification

	app2 := clickhelp.MustParseAppId("com.example.test_test-app-2_0")
	// (re-adding a couple)
	c.Assert(ec.Present(ecs.app, "notif7", f("one", 1, true)), Equals, true)
	c.Assert(ec.Present(app2, "notif8", f("two", 1, true)), Equals, true)
	c.Assert(ec.Tags(ecs.app), DeepEquals, []string{"one"})
	c.Assert(ec.Tags(app2), DeepEquals, []string{"two"})

	// asking to clear a non-existent tag does nothing
	c.Check(ec.Clear(app2, "foo"), Equals, 0)
	c.Check(ec.Tags(ecs.app), HasLen, 1)
	c.Check(ec.Tags(app2), HasLen, 1)

	// asking to clear a tag that exists only for another app does nothing
	c.Check(ec.Clear(app2, "one"), Equals, 0)
	c.Check(ec.Tags(ecs.app), HasLen, 1)
	c.Check(ec.Tags(app2), HasLen, 1)

	// asking to clear a list of tags, only one of which is yours, only clears yours
	c.Check(ec.Clear(app2, "one", "two"), Equals, 1)
	c.Check(ec.Tags(ecs.app), HasLen, 1)
	c.Check(ec.Tags(app2), IsNil)

	// asking to clear all the tags from an already tagless app does nothing
	c.Check(ec.Clear(app2), Equals, 0)
	c.Check(ec.Tags(ecs.app), HasLen, 1)
	c.Check(ec.Tags(app2), IsNil)

	// clearing with no args just empties it
	c.Check(ec.Clear(ecs.app), Equals, 1)
	c.Check(ec.Clear(ecs.app), Equals, 0)

	// check we work ok with a "" tag, too.
	app3 := clickhelp.MustParseAppId("com.example.test_test-app-3_0")
	c.Assert(ec.Present(app3, "notif9", f("", 1, true)), Equals, true)
	c.Check(ec.Tags(app3), DeepEquals, []string{""})
	c.Check(ec.Clear(app3, ""), Equals, 1)
	c.Check(ec.Tags(app3), IsNil)
}
