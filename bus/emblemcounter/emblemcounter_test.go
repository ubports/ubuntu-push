package emblemcounter

import (
	"testing"

	"launchpad.net/go-dbus/v1"
	. "launchpad.net/gocheck"

	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/launch_helper"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

func TestClient(t *testing.T) { TestingT(t) }

type ecSuite struct {
	log *helpers.TestLogger
}

var _ = Suite(&ecSuite{})

func (ecs *ecSuite) SetUpTest(c *C) {
	ecs.log = helpers.NewTestLogger(c, "debug")
}

// checks that Present() actually calls SetProperty on the launcher
func (ecs *ecSuite) TestPresentPresents(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))

	ec := New(endp, ecs.log)
	notif := launch_helper.Notification{EmblemCounter: &launch_helper.EmblemCounter{Count: 42, Visible: true}}
	ec.Present("com.example.test_test-app", "nid", &notif)
	callArgs := testibus.GetCallArgs(endp)
	c.Assert(callArgs, HasLen, 2)
	c.Check(callArgs[0].Member, Equals, "::SetProperty")
	c.Check(callArgs[1].Member, Equals, "::SetProperty")
	c.Check(callArgs[0].Args, DeepEquals, []interface{}{"count", "/test_2dapp", dbus.Variant{Value: int32(42)}})
	c.Check(callArgs[1].Args, DeepEquals, []interface{}{"countVisible", "/test_2dapp", dbus.Variant{Value: true}})
}

// check that Present() doesn't call SetProperty if no EmblemCounter in the Notification
func (ecs *ecSuite) TestSkipIfMissing(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))
	ec := New(endp, ecs.log)

	// nothing happens if nil Notification
	ec.Present("com.example.test_test-app", "nid", nil)
	c.Assert(testibus.GetCallArgs(endp), HasLen, 0)

	// nothing happens if no EmblemCounter in Notification
	ec.Present("com.example.test_test-app", "nid", &launch_helper.Notification{})
	c.Assert(testibus.GetCallArgs(endp), HasLen, 0)

	// but an empty EmblemCounter is acted on
	ec.Present("com.example.test_test-app", "nid", &launch_helper.Notification{EmblemCounter: &launch_helper.EmblemCounter{}})
	callArgs := testibus.GetCallArgs(endp)
	c.Assert(callArgs, HasLen, 2)
	c.Check(callArgs[0].Member, Equals, "::SetProperty")
	c.Check(callArgs[1].Member, Equals, "::SetProperty")
	c.Check(callArgs[0].Args, DeepEquals, []interface{}{"count", "/test_2dapp", dbus.Variant{Value: int32(0)}})
	c.Check(callArgs[1].Args, DeepEquals, []interface{}{"countVisible", "/test_2dapp", dbus.Variant{Value: false}})
}
