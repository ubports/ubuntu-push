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

// Package messaging wraps the messaging menu indicator, allowing for persistent
// notifications to the user.
package messaging

import (
	"encoding/json"
	"sync"
	"time"

	"launchpad.net/ubuntu-push/bus/notifications"
	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/messaging/cmessaging"
	"launchpad.net/ubuntu-push/messaging/reply"
)

type MessagingMenu struct {
	Log             logger.Logger
	Ch              chan *reply.MMActionReply
	notifications   map[string]*cmessaging.Payload // keep a ref to the Payload used in the MMU callback
	lock            sync.RWMutex
	lastCleanupTime time.Time
}

type cleanUp func()

// New returns a new MessagingMenu
func New(log logger.Logger) *MessagingMenu {
	return &MessagingMenu{Log: log, Ch: make(chan *reply.MMActionReply), notifications: make(map[string]*cmessaging.Payload)}
}

var cAddNotification = cmessaging.AddNotification
var cRemoveNotification = cmessaging.RemoveNotification
var cNotificationExists = cmessaging.NotificationExists

// GetCh returns the reply channel, exactly like mm.Ch.
func (mmu *MessagingMenu) GetCh() chan *reply.MMActionReply {
	return mmu.Ch
}

func (mmu *MessagingMenu) addNotification(app *click.AppId, notificationId string, tag string, card *launch_helper.Card, actions []string, testingCleanUpFunction cleanUp) {
	mmu.lock.Lock()
	defer mmu.lock.Unlock()
	payload := &cmessaging.Payload{Ch: mmu.Ch, Actions: actions, App: app, Tag: tag}
	mmu.notifications[notificationId] = payload
	cAddNotification(app.DesktopId(), notificationId, card, payload)

	// Clean up our internal notifications store if it holds more than 20 messages (and apparently nobody ever calls Tags())
	if len(mmu.notifications) > 20 && time.Since(mmu.lastCleanupTime).Minutes() > 10 {
		mmu.lastCleanupTime = time.Now()
		if testingCleanUpFunction == nil {
			go mmu.cleanUpNotifications()
		} else {
			testingCleanUpFunction() // Has to implement the asynchronous part itself
		}

	}
}

func (mmu *MessagingMenu) RemoveNotification(notificationId string, fromUI bool) {
	mmu.lock.Lock()
	defer mmu.lock.Unlock()
	payload := mmu.notifications[notificationId]
	delete(mmu.notifications, notificationId)
	if payload != nil && payload.App != nil && fromUI {
		cRemoveNotification(payload.App.DesktopId(), notificationId)
	}
}

func (mmu *MessagingMenu) cleanUpNotifications() {
	mmu.lock.Lock()
	defer mmu.lock.Unlock()
	mmu.doCleanUpNotifications()
}

// doCleanupNotifications removes notifications that were cleared from the messaging menu
func (mmu *MessagingMenu) doCleanUpNotifications() {
	for nid, payload := range mmu.notifications {
		if !cNotificationExists(payload.App.DesktopId(), nid) {
			delete(mmu.notifications, nid)
		}
	}
}

func (mmu *MessagingMenu) Tags(app *click.AppId) []string {
	orig := app.Original()
	tags := []string(nil)
	mmu.lock.Lock()
	defer mmu.lock.Unlock()
	mmu.lastCleanupTime = time.Now()
	mmu.doCleanUpNotifications()
	for _, payload := range mmu.notifications {
		if payload.App.Original() == orig {
			tags = append(tags, payload.Tag)
		}
	}
	return tags
}

func (mmu *MessagingMenu) Clear(app *click.AppId, tags ...string) int {
	orig := app.Original()
	var nids []string

	mmu.lock.RLock()
	// O(n×m). Should be small n and m though.
	for nid, payload := range mmu.notifications {
		if payload.App.Original() == orig {
			if len(tags) == 0 {
				nids = append(nids, nid)
			} else {
				for _, tag := range tags {
					if payload.Tag == tag {
						nids = append(nids, nid)
					}
				}
			}
		}
	}
	mmu.lock.RUnlock()

	for _, nid := range nids {
		mmu.RemoveNotification(nid, true)
	}
	mmu.cleanUpNotifications()

	return len(nids)
}

func (mmu *MessagingMenu) Present(app *click.AppId, nid string, notification *launch_helper.Notification) bool {
	if notification == nil {
		panic("please check notification is not nil before calling present")
	}

	card := notification.Card

	if card == nil || !card.Persist || card.Summary == "" {
		mmu.Log.Debugf("[%s] notification has no persistable card: %#v", nid, card)
		return false
	}

	actions := make([]string, 2*len(card.Actions))
	for i, action := range card.Actions {
		act, err := json.Marshal(&notifications.RawAction{
			App:      app,
			Nid:      nid,
			ActionId: i,
			Action:   action,
		})
		if err != nil {
			mmu.Log.Errorf("failed to build action: %s", action)
			return false
		}
		actions[2*i] = string(act)
		actions[2*i+1] = action
	}

	mmu.Log.Debugf("[%s] creating notification centre entry for %s (summary: %s)", nid, app.Base(), card.Summary)

	mmu.addNotification(app, nid, notification.Tag, card, actions, nil)

	return true
}
