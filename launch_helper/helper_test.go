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

package launch_helper

import (
	"testing"

	. "launchpad.net/gocheck"

	helpers "launchpad.net/ubuntu-push/testing"
)

func Test(t *testing.T) { TestingT(t) }

type runnerSuite struct {
	testlog *helpers.TestLogger
}

var _ = Suite(&runnerSuite{})

func (s *runnerSuite) SetUpTest(c *C) {
	s.testlog = helpers.NewTestLogger(c, "error")
}

func (s *runnerSuite) TestTrivialRunnerWorks(c *C) {
	notif := &Notification{Sound: "42"}

	triv := NewTrivialHelperLauncher(s.testlog)
	// []byte is sent as a base64-encoded string
	out := triv.Run("foo", []byte(`{"message": "aGVsbG8=", "notification": {"sound": "42"}}`))
	c.Assert(out, NotNil)
	c.Check(out.Message, DeepEquals, []byte("hello"))
	c.Check(out.Notification, DeepEquals, notif)
}

func (s *runnerSuite) TestTrivialRunnerWorksOnBadInput(c *C) {
	triv := NewTrivialHelperLauncher(s.testlog)
	msg := []byte(`this is a not your grandmother's json message`)
	out := triv.Run("foo", msg)
	c.Assert(out, NotNil)
	c.Check(out.Notification, IsNil)
	c.Check(out.Message, DeepEquals, msg)
}

func (s *runnerSuite) TestTrivialRunnerWorksOnBadFormat(c *C) {
	triv := NewTrivialHelperLauncher(s.testlog)
	msg := []byte(`{"url": "foo/bar"}`)
	out := triv.Run("foo", msg)
	c.Assert(out, NotNil)
	c.Check(out.Notification, IsNil)
	c.Check(out.Message, DeepEquals, msg)
}
