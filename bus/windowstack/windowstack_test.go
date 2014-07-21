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

package windowstack

import (
	"testing"

	. "launchpad.net/gocheck"

	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/click"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

func TestWindowStack(t *testing.T) { TestingT(t) }

type stackSuite struct {
	log *helpers.TestLogger
	app *click.AppId
}

var _ = Suite(&stackSuite{})

func (hs *stackSuite) SetUpTest(c *C) {
	hs.log = helpers.NewTestLogger(c, "debug")
	hs.app = clickhelp.MustParseAppId("com.example.test_test-app_0")
}

// Checks that GetWindowStack() actually calls GetWindowStack
func (ss *stackSuite) TestGetsWindowStack(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), []WindowsInfo{})
	ec := New(endp, ss.log)
	c.Check(ec.GetWindowStack(), DeepEquals, []WindowsInfo{})
	callArgs := testibus.GetCallArgs(endp)
	c.Assert(callArgs, HasLen, 1)
	c.Check(callArgs[0].Member, Equals, "GetWindowStack")
	c.Check(callArgs[0].Args, DeepEquals, []interface{}(nil))
}

var isFocusedTests = []struct {
	expected bool          // expected result
	wstack   []WindowsInfo // window stack data
}{
	{
		false,
		[]WindowsInfo{}, // No windows
	},
	{
		true,
		[]WindowsInfo{{0, "com.example.test_test-app", true, 0}}, // Just one window, matching app
	},
	{
		false,
		[]WindowsInfo{{0, "com.example.test_notest-app", true, 0}}, // Just one window, not matching app
	},
	{
		true,
		[]WindowsInfo{{0, "com.example.test_notest-app", false, 0}, {0, "com.example.test_test-app", true, 0}}, // Two windows, app focused
	},
	{
		false,
		[]WindowsInfo{{0, "com.example.test_notest-app", true, 0}, {0, "com.example.test_test-app", false, 0}}, // Two windows, app unfocused
	},
	{
		true,
		[]WindowsInfo{{0, "com.example.test_notest-app", true, 0}, {0, "com.example.test_test-app", true, 0}}, // Two windows, both focused
	},
}

// Check that if the app is focused, IsAppFocused returns true
func (ss *stackSuite) TestIsAppFocused(c *C) {
	for _, t := range isFocusedTests {
		endp := testibus.NewTestingEndpoint(nil, condition.Work(true), t.wstack)
		ec := New(endp, ss.log)
		c.Check(ec.IsAppFocused(ss.app), Equals, t.expected)
	}
}
