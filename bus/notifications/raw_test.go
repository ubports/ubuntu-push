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

// Package notifications wraps a couple of Notifications's DBus API points:
// the org.freedesktop.Notifications.Notify call, and listening for the
// ActionInvoked signal.
package notifications

import (
	. "launchpad.net/gocheck"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
	"testing"
	"time"
)

// hook up gocheck
func TestRaw(t *testing.T) { TestingT(t) }

type RawSuite struct {
	log logger.Logger
}

func (s *RawSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
}

var _ = Suite(&RawSuite{})

func (s *RawSuite) TestNotifies(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), uint32(1))
	raw := Raw(endp, s.log)
	nid, err := raw.Notify("", 0, "", "", "", nil, nil, 0)
	c.Check(err, IsNil)
	c.Check(nid, Equals, uint32(1))
}

func (s *RawSuite) TestNotifiesFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	raw := Raw(endp, s.log)
	_, err := raw.Notify("", 0, "", "", "", nil, nil, 0)
	c.Check(err, NotNil)
}

func (s *RawSuite) TestNotifyFailsIfNoBus(c *C) {
	raw := Raw(nil, s.log)
	_, err := raw.Notify("", 0, "", "", "", nil, nil, 0)
	c.Check(err, ErrorMatches, `.*unconfigured .*`)
}

func (s *RawSuite) TestNotifiesFailsWeirdly(c *C) {
	endp := testibus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{1, 2})
	raw := Raw(endp, s.log)
	_, err := raw.Notify("", 0, "", "", "", nil, nil, 0)
	c.Check(err, NotNil)
}

func (s *RawSuite) TestWatchActions(c *C) {
	endp := testibus.NewMultiValuedTestingEndpoint(nil, condition.Work(true),
		[]interface{}{uint32(1), "hello"})
	raw := Raw(endp, s.log)
	ch, err := raw.WatchActions()
	c.Assert(err, IsNil)
	// check we get the right action reply
	select {
	case p := <-ch:
		c.Check(p.NotificationId, Equals, uint32(1))
		c.Check(p.ActionId, Equals, "hello")
	case <-time.NewTimer(time.Second / 10).C:
		c.Error("timed out?")
	}
	// and that the channel is closed if/when the watch fails
	_, ok := <-ch
	c.Check(ok, Equals, false)
}

func (s *RawSuite) TestWatchActionsFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	raw := Raw(endp, s.log)
	_, err := raw.WatchActions()
	c.Check(err, NotNil)
}
