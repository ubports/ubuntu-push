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

var cleanupLoopDuration = 5 * time.Minute

type MessagingMenu struct {
	Log               logger.Logger
	Ch                chan *reply.MMActionReply
	notifications     map[string]*cmessaging.Payload // keep a ref to the Payload used in the MMU callback
	lock              sync.RWMutex
	stopCleanupLoopCh chan bool
	ticker            *time.Ticker
	tickerCh          <-chan time.Time
}

// New returns a new MessagingMenu
func New(log logger.Logger) *MessagingMenu {
	ticker := time.NewTicker(cleanupLoopDuration)
	stopCh := make(chan bool)
	return &MessagingMenu{Log: log, Ch: make(chan *reply.MMActionReply), notifications: make(map[string]*cmessaging.Payload), ticker: ticker, tickerCh: ticker.C, stopCleanupLoopCh: stopCh}
}

var cAddNotification = cmessaging.AddNotification
var cNotificationExists = cmessaging.NotificationExists

func (mmu *MessagingMenu) addNotification(app *click.AppId, notificationId string, tag string, card *launch_helper.Card, actions []string) {
	mmu.lock.Lock()
	defer mmu.lock.Unlock()
	payload := &cmessaging.Payload{Ch: mmu.Ch, Actions: actions, App: app, Tag: tag}
	mmu.notifications[notificationId] = payload
	cAddNotification(app.DesktopId(), notificationId, card, payload)
}

// RemoveNotification deletes the notification from internal map
func (mmu *MessagingMenu) RemoveNotification(notificationId string) {
	mmu.lock.Lock()
	defer mmu.lock.Unlock()
	delete(mmu.notifications, notificationId)
}

// cleanupNotifications remove notifications that were cleared from the messaging menu
func (mmu *MessagingMenu) cleanUpNotifications() {
	mmu.lock.Lock()
	defer mmu.lock.Unlock()
	for nid, payload := range mmu.notifications {
		if payload.Gone {
			// sweep
			delete(mmu.notifications, nid)
			// don't check the mmu for this nid
			continue
		}
		exists := cNotificationExists(payload.App.DesktopId(), nid)
		if !exists {
			// mark
			payload.Gone = true
		}
	}
}

func (mmu *MessagingMenu) StartCleanupLoop() {
	mmu.doStartCleanupLoop(mmu.cleanUpNotifications)
}

func (mmu *MessagingMenu) doStartCleanupLoop(cleanupFunc func()) {
	go func() {
		for {
			select {
			case <-mmu.tickerCh:
				cleanupFunc()
			case <-mmu.stopCleanupLoopCh:
				mmu.ticker.Stop()
				mmu.Log.Debugf("CleanupLoop stopped.")
				return
			}
		}
	}()
}

func (mmu *MessagingMenu) StopCleanupLoop() {
	mmu.stopCleanupLoopCh <- true
}

func (mmu *MessagingMenu) Tags(app *click.AppId) []string {
	orig := app.Original()
	tags := []string(nil)
	mmu.lock.RLock()
	defer mmu.lock.RUnlock()
	for _, payload := range mmu.notifications {
		if payload.App.Original() == orig {
			tags = append(tags, payload.Tag)
		}
	}
	return tags
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
			mmu.Log.Errorf("Failed to build action: %s", action)
			return false
		}
		actions[2*i] = string(act)
		actions[2*i+1] = action
	}

	mmu.Log.Debugf("[%s] creating notification centre entry for %s (summary: %s)", nid, app.Base(), card.Summary)

	mmu.addNotification(app, nid, notification.Tag, card, actions)

	return true
}
