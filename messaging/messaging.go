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
	notifications     map[string]*cmessaging.Payload
	lock              sync.RWMutex
	stopCleanupLoopCh chan bool
	ticker            *time.Ticker
}

// New returns a new MessagingMenu
func New(log logger.Logger) *MessagingMenu {
	ticker := time.NewTicker(cleanupLoopDuration)
	stopCh := make(chan bool)
	return &MessagingMenu{Log: log, Ch: make(chan *reply.MMActionReply), notifications: make(map[string]*cmessaging.Payload), ticker: ticker, stopCleanupLoopCh: stopCh}
}

var cAddNotification = cmessaging.AddNotification
var cNotificationExists = cmessaging.NotificationExists

func (mmu *MessagingMenu) addNotification(desktopId string, notificationId string, tag string, card *launch_helper.Card, actions []string) {
	payload := &cmessaging.Payload{Ch: mmu.Ch, Actions: actions, DesktopId: desktopId, Tag: tag}
	mmu.lock.Lock()
	// XXX: only gets removed if the action is activated.
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
		if !cNotificationExists(payload.DesktopId, nid) {
			delete(mmu.notifications, nid)
		}
	}
}

func (mmu *MessagingMenu) StartCleanupLoop() {
	go func() {
		for {
			select {
			case <-mmu.ticker.C:
				mmu.cleanUpNotifications()
			case <-mmu.stopCleanupLoopCh:
				mmu.ticker.Stop()
				return
			}
		}
	}()
}

func (mmu *MessagingMenu) StopCleanupLoop() {
	mmu.stopCleanupLoopCh <- true
}

func (mmu *MessagingMenu) Tags(app *click.AppId) map[string][]string {
	desktopId := app.DesktopId()
	tags := []string(nil)
	mmu.lock.RLock()
	defer mmu.lock.RUnlock()
	for _, payload := range mmu.notifications {
		if payload.DesktopId == desktopId {
			tags = append(tags, payload.Tag)
		}
	}
	if tags == nil {
		return nil
	}
	return map[string][]string{"card": tags}
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

	mmu.addNotification(app.DesktopId(), nid, notification.Tag, card, actions)

	return true
}
