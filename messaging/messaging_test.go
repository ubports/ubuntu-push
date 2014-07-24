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

package messaging

import (
	"time"

	. "launchpad.net/gocheck"
	"testing"

	"launchpad.net/ubuntu-push/click"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/messaging/cmessaging"
	helpers "launchpad.net/ubuntu-push/testing"
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type MessagingSuite struct {
	log *helpers.TestLogger
	app *click.AppId
}

var _ = Suite(&MessagingSuite{})

func (ms *MessagingSuite) SetUpSuite(c *C) {
	cAddNotification = func(a string, n string, c *launch_helper.Card, payload *cmessaging.Payload) {
		ms.log.Debugf("ADD: app: %s, not: %s, card: %v, chan: %v", a, n, c, payload)
	}
	cRemoveNotification = func(a, n string) {
		ms.log.Debugf("REMOVE: app: %s, not: %s", a, n)
	}
	// just in case
	cNotificationExists = nil
}

func (ms *MessagingSuite) TearDownSuite(c *C) {
	cAddNotification = cmessaging.AddNotification
	cRemoveNotification = cmessaging.RemoveNotification
	cNotificationExists = cmessaging.NotificationExists
}

func (ms *MessagingSuite) SetUpTest(c *C) {
	ms.log = helpers.NewTestLogger(c, "debug")
	ms.app = clickhelp.MustParseAppId("com.example.test_test_0")
}

func (ms *MessagingSuite) TestPresentPresents(c *C) {
	mmu := New(ms.log)
	card := launch_helper.Card{Summary: "ehlo", Persist: true}
	notif := launch_helper.Notification{Card: &card}

	c.Check(mmu.Present(ms.app, "notif-id", &notif), Equals, true)

	c.Check(ms.log.Captured(), Matches, `(?s).* ADD:.*notif-id.*`)
}

func (ms *MessagingSuite) TestPresentDoesNotPresentsIfNoSummary(c *C) {
	mmu := New(ms.log)
	card := launch_helper.Card{Persist: true}
	notif := launch_helper.Notification{Card: &card}

	c.Check(mmu.Present(ms.app, "notif-id", &notif), Equals, false)

	c.Check(ms.log.Captured(), Matches, "(?sm).*has no persistable card.*")
}

func (ms *MessagingSuite) TestPresentDoesNotPresentsIfNotPersist(c *C) {
	mmu := New(ms.log)
	card := launch_helper.Card{Summary: "ehlo"}
	notif := launch_helper.Notification{Card: &card}

	c.Check(mmu.Present(ms.app, "notif-id", &notif), Equals, false)

	c.Check(ms.log.Captured(), Matches, "(?sm).*has no persistable card.*")
}

func (ms *MessagingSuite) TestPresentPanicsIfNil(c *C) {
	mmu := New(ms.log)
	c.Check(func() { mmu.Present(ms.app, "notif-id", nil) }, Panics, `please check notification is not nil before calling present`)
}

func (ms *MessagingSuite) TestPresentDoesNotPresentsIfNilCard(c *C) {
	mmu := New(ms.log)
	c.Check(mmu.Present(ms.app, "notif-id", &launch_helper.Notification{}), Equals, false)
	c.Check(ms.log.Captured(), Matches, "(?sm).*no persistable card.*")
}

func (ms *MessagingSuite) TestPresentWithActions(c *C) {
	mmu := New(ms.log)
	card := launch_helper.Card{Summary: "ehlo", Persist: true, Actions: []string{"action-1"}}
	notif := launch_helper.Notification{Card: &card, Tag: "a-tag"}

	c.Check(mmu.Present(ms.app, "notif-id", &notif), Equals, true)

	c.Check(ms.log.Captured(), Matches, `(?s).* ADD:.*notif-id.*`)

	payload, _ := mmu.notifications["notif-id"]
	c.Check(payload.Ch, Equals, mmu.Ch)
	c.Check(len(payload.Actions), Equals, 2)
	c.Check(payload.Tag, Equals, "a-tag")
	rawAction := "{\"app\":\"com.example.test_test_0\",\"act\":\"action-1\",\"nid\":\"notif-id\"}"
	c.Check(payload.Actions[0], Equals, rawAction)
	c.Check(payload.Actions[1], Equals, "action-1")
}

func (ms *MessagingSuite) TestTagsListsTags(c *C) {
	mmu := New(ms.log)
	f := func(s string) *launch_helper.Notification {
		card := launch_helper.Card{Summary: "tag: \"" + s + "\"", Persist: true}
		return &launch_helper.Notification{Card: &card, Tag: s}
	}

	c.Check(mmu.Tags(ms.app), IsNil)
	c.Assert(mmu.Present(ms.app, "notif1", f("one")), Equals, true)
	c.Check(mmu.Tags(ms.app), DeepEquals, []string{"one"})
	c.Assert(mmu.Present(ms.app, "notif2", f("")), Equals, true)
	c.Check(mmu.Tags(ms.app), DeepEquals, []string{"one", ""})
	// and an empty notification doesn't count
	c.Assert(mmu.Present(ms.app, "notif3", &launch_helper.Notification{Tag: "X"}), Equals, false)
	c.Check(mmu.Tags(ms.app), DeepEquals, []string{"one", ""})
	// and they go away if we remove one
	mmu.RemoveNotification("notif1")
	c.Check(mmu.Tags(ms.app), DeepEquals, []string{""})
	mmu.RemoveNotification("notif2")
	c.Check(mmu.Tags(ms.app), IsNil)
}

func (ms *MessagingSuite) TestClearClears(c *C) {
	app1 := ms.app
	app2 := clickhelp.MustParseAppId("com.example.test_test-2_0")
	app3 := clickhelp.MustParseAppId("com.example.test_test-3_0")
	mm := New(ms.log)
	f := func(app *click.AppId, nid string, tag string, withCard bool) bool {
		notif := launch_helper.Notification{Tag: tag}
		card := launch_helper.Card{Summary: "tag: \"" + tag + "\"", Persist: true}
		if withCard {
			notif.Card = &card
		}
		return mm.Present(app, nid, &notif)
	}
	// create a bunch
	c.Assert(f(app1, "notif1", "one", true), Equals, true)
	c.Assert(f(app1, "notif2", "two", true), Equals, true)
	c.Assert(f(app1, "notif3", "", true), Equals, true)
	c.Assert(f(app2, "notif4", "one", true), Equals, true)
	c.Assert(f(app2, "notif5", "two", true), Equals, true)
	c.Assert(f(app3, "notif6", "one", true), Equals, true)
	c.Assert(f(app3, "notif7", "", true), Equals, true)

	// that is:
	//   app 1: "one", "two", "";
	//   app 2: "one", "two";
	//   app 3: "one", ""
	c.Assert(mm.Tags(app1), DeepEquals, []string{"one", "two", ""})
	c.Assert(mm.Tags(app2), DeepEquals, []string{"one", "two"})
	c.Assert(mm.Tags(app3), DeepEquals, []string{"one", ""})

	// clearing a non-existent tag does nothing
	c.Check(mm.Clear(app1, "foo"), Equals, 0)
	c.Check(mm.Tags(app1), HasLen, 3)
	c.Check(mm.Tags(app2), HasLen, 2)
	c.Check(mm.Tags(app3), HasLen, 2)

	// asking to clear a tag that exists only for another app does nothing
	c.Check(mm.Clear(app3, "two"), Equals, 0)
	c.Check(mm.Tags(app1), HasLen, 3)
	c.Check(mm.Tags(app2), HasLen, 2)
	c.Check(mm.Tags(app3), HasLen, 2)

	// asking to clear a list of tags, only one of which is yours, only clears yours
	c.Check(mm.Clear(app3, "one", "two"), Equals, 1)
	c.Check(mm.Tags(app1), HasLen, 3)
	c.Check(mm.Tags(app2), HasLen, 2)
	c.Check(mm.Tags(app3), HasLen, 1)

	// clearing with no args just empties it
	c.Check(mm.Clear(app1), Equals, 3)
	c.Check(mm.Tags(app1), IsNil)
	c.Check(mm.Tags(app2), HasLen, 2)
	c.Check(mm.Tags(app3), HasLen, 1)

	// asking to clear all the tags from an already tagless app does nothing
	c.Check(mm.Clear(app1), Equals, 0)
	c.Check(mm.Tags(app1), IsNil)
	c.Check(mm.Tags(app2), HasLen, 2)
	c.Check(mm.Tags(app3), HasLen, 1)

	// check we work ok with a "" tag, too.
	c.Check(mm.Clear(app1, ""), Equals, 0)
	c.Check(mm.Clear(app2, ""), Equals, 0)
	c.Check(mm.Clear(app3, ""), Equals, 1)
	c.Check(mm.Tags(app1), IsNil)
	c.Check(mm.Tags(app2), HasLen, 2)
	c.Check(mm.Tags(app3), HasLen, 0)
}

func (ms *MessagingSuite) TestRemoveNotification(c *C) {
	mmu := New(ms.log)
	card := launch_helper.Card{Summary: "ehlo", Persist: true, Actions: []string{"action-1"}}
	actions := []string{"{\"app\":\"com.example.test_test_0\",\"act\":\"action-1\",\"nid\":\"notif-id\"}", "action-1"}
	mmu.addNotification(ms.app, "notif-id", "a-tag", &card, actions)

	// check it's there
	payload, ok := mmu.notifications["notif-id"]
	c.Check(ok, Equals, true)
	c.Check(payload.Actions, DeepEquals, actions)
	c.Check(payload.Tag, Equals, "a-tag")
	c.Check(payload.Ch, Equals, mmu.Ch)
	// remove the notification (internal only)
	mmu.removeNotification("notif-id", false)
	// check it's gone
	_, ok = mmu.notifications["notif-id"]
	c.Check(ok, Equals, false)
}

func (ms *MessagingSuite) TestRemoveNotificationsFromUI(c *C) {
	mmu := New(ms.log)
	card := launch_helper.Card{Summary: "ehlo", Persist: true, Actions: []string{"action-1"}}
	actions := []string{"{\"app\":\"com.example.test_test_0\",\"act\":\"action-1\",\"nid\":\"notif-id\"}", "action-1"}
	mmu.addNotification(ms.app, "notif-id", "a-tag", &card, actions)

	// check it's there
	_, ok := mmu.notifications["notif-id"]
	c.Assert(ok, Equals, true)
	// remove the notification (both internal and from UI)
	mmu.removeNotification("notif-id", true)
	// check it's gone
	_, ok = mmu.notifications["notif-id"]
	c.Check(ok, Equals, false)

	// and check it's been removed from the UI too
	c.Check(ms.log.Captured(), Matches, `(?s).* REMOVE:.*notif-id.*`)
}

func (ms *MessagingSuite) TestCleanupStaleNotification(c *C) {
	mmu := New(ms.log)
	card := launch_helper.Card{Summary: "ehlo", Persist: true, Actions: []string{"action-1"}}
	actions := []string{"{\"app\":\"com.example.test_test_0\",\"act\":\"action-1\",\"nid\":\"notif-id\"}", "action-1"}
	mmu.addNotification(ms.app, "notif-id", "", &card, actions)

	// check it's there
	_, ok := mmu.notifications["notif-id"]
	c.Check(ok, Equals, true)

	// patch cnotificationexists to return true
	cNotificationExists = func(did string, nid string) bool {
		return true
	}
	// remove the notification
	mmu.cleanUpNotifications()
	// check it's still there
	_, ok = mmu.notifications["notif-id"]
	c.Check(ok, Equals, true)
	// patch cnotificationexists to return false
	cNotificationExists = func(did string, nid string) bool {
		return false
	}
	// mark the notification
	mmu.cleanUpNotifications()
	// check it's gone
	_, ok = mmu.notifications["notif-id"]
	c.Check(ok, Equals, true)
	// sweep the notification
	mmu.cleanUpNotifications()
	// check it's gone
	_, ok = mmu.notifications["notif-id"]
	c.Check(ok, Equals, false)
}

func (ms *MessagingSuite) TestCleanupLoop(c *C) {
	mmu := New(ms.log)
	tickerCh := make(chan time.Time)
	mmu.tickerCh = tickerCh
	cleanupCh := make(chan bool)
	cleanupFunc := func() {
		cleanupCh <- true
	}
	// start the cleanup loop
	mmu.doStartCleanupLoop(cleanupFunc)
	// mark
	tickerCh <- time.Now()
	// check it was called
	<-cleanupCh
	// stop the loop and check that it's actually stopped.
	mmu.StopCleanupLoop()
	c.Check(ms.log.Captured(), Matches, "(?s).*DEBUG CleanupLoop stopped.*")
}

func (ms *MessagingSuite) TestStartCleanupLoop(c *C) {
	mmu := New(ms.log)
	tickerCh := make(chan time.Time)
	mmu.tickerCh = tickerCh
	card := launch_helper.Card{Summary: "ehlo", Persist: true, Actions: []string{"action-1"}}
	actions := []string{"{\"app\":\"com.example.test_test_0\",\"act\":\"action-1\",\"nid\":\"notif-id\"}", "action-1"}
	mmu.addNotification(ms.app, "notif-id", "", &card, actions)
	// patch cnotificationexists to return true and signal when it's called
	notifExistsCh := make(chan bool)
	cNotificationExists = func(did string, nid string) bool {
		notifExistsCh <- true
		return true
	}
	// statr the cleanup loop
	mmu.StartCleanupLoop()
	// mark
	tickerCh <- time.Now()
	// check it's there, and marked
	<-notifExistsCh
	// stop the loop
	mmu.StopCleanupLoop()
}
