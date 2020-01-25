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

	"launchpad.net/go-dbus"
	. "launchpad.net/gocheck"

	testibus "github.com/ubports/ubuntu-push/bus/testing"
	"github.com/ubports/ubuntu-push/click"
	clickhelp "github.com/ubports/ubuntu-push/click/testing"
	"github.com/ubports/ubuntu-push/launch_helper"
	"github.com/ubports/ubuntu-push/nih"
	helpers "github.com/ubports/ubuntu-push/testing"
	"github.com/ubports/ubuntu-push/testing/condition"
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
	c.Check(ec.SetCounter(ecs.app, 42, true), Equals, true)
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
