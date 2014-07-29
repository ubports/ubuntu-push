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
	"encoding/json"
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

func (s *runnerSuite) TestTrivialPoolWorks(c *C) {
	notif := &Notification{Sound: "42", Tag: "foo"}

	triv := NewTrivialHelperPool(s.testlog)
	ch := triv.Start()
	in := &HelperInput{App: s.app, Payload: []byte(`{"message": {"m":42}, "notification": {"sound": "42", "tag": "foo"}}`)}
	triv.Run("klick", in)
	out := <-ch
	c.Assert(out, NotNil)
	c.Check(out.Message, DeepEquals, json.RawMessage(`{"m":42}`))
	c.Check(out.Notification, DeepEquals, notif)
	c.Check(out.Input, DeepEquals, in)
}

func (s *runnerSuite) TestTrivialPoolWorksOnBadInput(c *C) {
	triv := NewTrivialHelperPool(s.testlog)
	ch := triv.Start()
	msg := []byte(`{card: 3}`)
	in := &HelperInput{App: s.app, Payload: msg}
	triv.Run("klick", in)
	out := <-ch
	c.Assert(out, NotNil)
	c.Check(out.Notification, IsNil)
	c.Check(out.Message, DeepEquals, json.RawMessage(msg))
	c.Check(out.Input, DeepEquals, in)
}

func (s *runnerSuite) TestTrivialPoolDoesNotBlockEasily(c *C) {
	triv := NewTrivialHelperPool(s.testlog)
	triv.Start()
	msg := []byte(`this is a not your grandmother's json message`)
	in := &HelperInput{App: s.app, Payload: msg}
	flagCh := make(chan bool)
	go func() {
		// stuff several in there
		triv.Run("klick", in)
		triv.Run("klick", in)
		triv.Run("klick", in)
		flagCh <- true
	}()
	select {
	case <-flagCh:
		// whee
	case <-time.After(10 * time.Millisecond):
		c.Fatal("runner blocked too easily")
	}
}
