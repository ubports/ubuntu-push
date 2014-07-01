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

func (mmu *MessagingMenu) addNotification(appId string, notificationId string, card *launch_helper.Card) {
	cAddNotification(appId, notificationId, card, mmu.Ch)
}

func (mmu *MessagingMenu) Present(appId string, notificationId string, notification *launch_helper.Notification) {
	if notification == nil || notification.Card == nil || !notification.Card.Persist || notification.Card.Summary == "" {
		return
	}

	mmu.addNotification(appId, notificationId, notification.Card)
}
