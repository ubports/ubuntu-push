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
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/click"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	helpers "launchpad.net/ubuntu-push/testing"
)

func Test(t *testing.T) { TestingT(t) }

type runnerSuite struct {
	testlog *helpers.TestLogger
	app     *click.AppId
}

var _ = Suite(&runnerSuite{})

func (s *runnerSuite) SetUpTest(c *C) {
	s.testlog = helpers.NewTestLogger(c, "error")
	s.app = clickhelp.MustParseAppId("com.example.test_test-app_0")
}

func (s *runnerSuite) TestTrivialRunnerWorks(c *C) {
	notif := &Notification{Sound: "42"}

	triv := NewTrivialHelperLauncher(s.testlog)
	ch := triv.Start()
	// []byte is sent as a base64-encoded string
	in := &HelperInput{App: s.app, Message: []byte(`{"message": "aGVsbG8=", "notification": {"sound": "42"}}`)}
	triv.Run(in)
	out := <-ch
	c.Assert(out, NotNil)
	c.Check(out.Message, DeepEquals, []byte("hello"))
	c.Check(out.Notification, DeepEquals, notif)
	c.Check(out.Input, DeepEquals, in)
}

func (s *runnerSuite) TestTrivialRunnerWorksOnBadInput(c *C) {
	triv := NewTrivialHelperLauncher(s.testlog)
	ch := triv.Start()
	msg := []byte(`this is a not your grandmother's json message`)
	in := &HelperInput{App: s.app, Message: msg}
	triv.Run(in)
	out := <-ch
	c.Assert(out, NotNil)
	c.Check(out.Notification, IsNil)
	c.Check(out.Message, DeepEquals, msg)
	c.Check(out.Input, DeepEquals, in)
}

func (s *runnerSuite) TestTrivialRunnerDoesNotBlockEasily(c *C) {
	triv := NewTrivialHelperLauncher(s.testlog)
	triv.Start()
	msg := []byte(`this is a not your grandmother's json message`)
	in := &HelperInput{App: s.app, Message: msg}
	flagCh := make(chan bool)
	go func() {
		// stuff several in there
		triv.Run(in)
		triv.Run(in)
		triv.Run(in)
		flagCh <- true
	}()
	select {
	case <-flagCh:
		// whee
	case <-time.After(10 * time.Millisecond):
		c.Fatal("runner blocked too easily")
	}
}
