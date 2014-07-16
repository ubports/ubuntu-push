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
}

func (ms *MessagingSuite) SetUpTest(c *C) {
	ms.log = helpers.NewTestLogger(c, "debug")
	ms.app = clickhelp.MustParseAppId("com.example.test_test_0")
}

func (ms *MessagingSuite) TestPresentPresents(c *C) {
	mmu := New(ms.log)
	card := launch_helper.Card{Summary: "ehlo", Persist: true}
	notif := launch_helper.Notification{Card: &card}

	mmu.Present(ms.app, "notif-id", &notif)

	c.Check(ms.log.Captured(), Matches, `(?s).* ADD:.*notif-id.*`)
}

func (ms *MessagingSuite) TestPresentDoesNotPresentsIfNoSummary(c *C) {
	mmu := New(ms.log)
	card := launch_helper.Card{Persist: true}
	notif := launch_helper.Notification{Card: &card}

	mmu.Present(ms.app, "notif-id", &notif)

	c.Check(ms.log.Captured(), Matches, "(?sm).*has no persistable card.*")
}

func (ms *MessagingSuite) TestPresentDoesNotPresentsIfNotPersist(c *C) {
	mmu := New(ms.log)
	card := launch_helper.Card{Summary: "ehlo"}
	notif := launch_helper.Notification{Card: &card}

	mmu.Present(ms.app, "notif-id", &notif)

	c.Check(ms.log.Captured(), Matches, "(?sm).*has no persistable card.*")
}

func (ms *MessagingSuite) TestPresentDoesNotPresentsIfNil(c *C) {
	mmu := New(ms.log)
	mmu.Present(ms.app, "notif-id", nil)
	c.Check(ms.log.Captured(), Matches, "(?sm).*no notification.*")
}

func (ms *MessagingSuite) TestPresentDoesNotPresentsIfNilCard(c *C) {
	mmu := New(ms.log)
	mmu.Present(ms.app, "notif-id", &launch_helper.Notification{})
	c.Check(ms.log.Captured(), Matches, "(?sm).*no notification.*")
}
