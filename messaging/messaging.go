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

	"launchpad.net/ubuntu-push/bus/notifications"
	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/messaging/cmessaging"
	"launchpad.net/ubuntu-push/messaging/reply"
)

type MessagingMenu struct {
	Log logger.Logger
	Ch  chan *reply.MMActionReply
}

// New returns a new MessagingMenu
func New(log logger.Logger) *MessagingMenu {
	return &MessagingMenu{Log: log, Ch: make(chan *reply.MMActionReply)}
}

var cAddNotification = cmessaging.AddNotification

func (mmu *MessagingMenu) addNotification(desktopId string, notificationId string, card *launch_helper.Card, actions []string) {
	cAddNotification(desktopId, notificationId, card, actions, mmu.Ch)
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
			continue
		}
		actions[2*i] = string(act)
		actions[2*i+1] = action
	}

	mmu.addNotification(app.DesktopId(), notificationId, notification.Card, actions)
}
