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

func (mmu *MessagingMenu) addNotification(desktopId string, notificationId string, card *launch_helper.Card, actions []string) {
	payload := &cmessaging.Payload{Ch: mmu.Ch, Actions: actions, DesktopId: desktopId}
	mmu.lock.Lock()
	mmu.notifications[notificationId] = payload
	mmu.lock.Unlock()
	cAddNotification(desktopId, notificationId, card, payload)
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
		exists := cNotificationExists(payload.DesktopId, nid)
		payload, ok := mmu.notifications[nid]
		if !exists && ok && payload.Alive {
			// mark
			payload.Alive = false
		} else if !exists && ok && !payload.Alive {
			// sweep
			delete(mmu.notifications, nid)
		}
	}
}

func (mmu *MessagingMenu) StartCleanupLoop() {
	go func() {
		for {
			select {
			case <-mmu.tickerCh:
				mmu.cleanUpNotifications()
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

func (mmu *MessagingMenu) Present(app *click.AppId, notificationId string, notification *launch_helper.Notification) {
	if notification == nil || notification.Card == nil || !notification.Card.Persist || notification.Card.Summary == "" {
		mmu.Log.Debugf("[%s] no notification or notification has no persistable card: %#v", notificationId, notification)
		return
	}
	actions := make([]string, 2*len(notification.Card.Actions))
	for i, action := range notification.Card.Actions {
		act, err := json.Marshal(&notifications.RawAction{
			App:      app,
			Nid:      notificationId,
			ActionId: i,
			Action:   action,
		})
		if err != nil {
			mmu.Log.Errorf("Failed to build action: %s", action)
			return
		}
		actions[2*i] = string(act)
		actions[2*i+1] = action
	}

	mmu.addNotification(app.DesktopId(), notificationId, notification.Card, actions)
}
