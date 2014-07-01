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

// this is the lower-level api

import (
	"errors"
	"fmt"
	"hash/crc32"

	"launchpad.net/go-dbus/v1"
	"launchpad.net/ubuntu-push/bus"
	c_helper "launchpad.net/ubuntu-push/bus/notifications/app_helper"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
)

// Notifications lives on a well-knwon bus.Address
var BusAddress bus.Address = bus.Address{
	Interface: "org.freedesktop.Notifications",
	Path:      "/org/freedesktop/Notifications",
	Name:      "org.freedesktop.Notifications",
}

/*****************************************************************
 *    RawNotifications
 */

// convenience type for the (uint32, string) ActionInvoked signal data
type RawActionReply struct {
	NotificationId uint32
	ActionId       string
}

// a raw notification provides a low-level interface to the f.d.o. dbus
// notifications api
type RawNotifications struct {
	bus bus.Endpoint
	log logger.Logger
}

// Raw returns a new RawNotifications that'll use the provided bus.Endpoint
func Raw(endp bus.Endpoint, log logger.Logger) *RawNotifications {
	return &RawNotifications{endp, log}
}

/*
   public methods
*/

// Notify fires a notification
func (raw *RawNotifications) Notify(
	app_name string, reuse_id uint32,
	icon, summary, body string,
	actions []string, hints map[string]*dbus.Variant,
	timeout int32) (uint32, error) {
	// that's a long argument list! Take a breather.
	//
	if raw.bus == nil {
		return 0, errors.New("unconfigured (missing bus)")
	}
	var res uint32
	err := raw.bus.Call("Notify", bus.Args(app_name, reuse_id, icon,
		summary, body, actions, hints, timeout), &res)
	if err != nil {
		return 0, err
	}
	return res, nil
}

// WatchActions listens for ActionInvoked signals from the notification daemon
// and sends them over the channel provided
func (raw *RawNotifications) WatchActions() (<-chan RawActionReply, error) {
	ch := make(chan RawActionReply)
	err := raw.bus.WatchSignal("ActionInvoked",
		func(ns ...interface{}) {
			ch <- RawActionReply{ns[0].(uint32), ns[1].(string)}
		}, func() { close(ch) })
	if err != nil {
		raw.log.Debugf("Failed to set up the watch: %s", err)
		return nil, err
	}
	return ch, nil
}

// ShowCard displays a given card.
//
// If card.Actions has 1 action, it's an interactive notification.
// If card.Actions has 2 or more actions, it will show as a snap decision.
//
// WatchActions will receive something like this in the ActionId field:
// appId::notificationId::action.Id
func (raw *RawNotifications) Present(appId string, notificationId string, notification *launch_helper.Notification) (uint32, error) {
	if notification == nil || notification.Card == nil || !notification.Card.Popup || notification.Card.Summary == "" {
		return 0, nil
	}

	card := notification.Card

	app_icon := c_helper.AppIconFromId(appId)
	reuse_id := crc32.ChecksumIEEE([]byte(notificationId)) // reuse the same bubble for the same notification
	hints := make(map[string]*dbus.Variant)
	hints["x-canonical-secondary-icon"] = &dbus.Variant{app_icon}

	actions := make([]string, 0, len(card.Actions))
	for i, action := range card.Actions {
		actions = append(actions, fmt.Sprintf("%s::%s::%d", appId, notificationId, i), action)
	}
	if len(actions) > 1 {
		hints["x-canonical-snap-decisions"] = &dbus.Variant{true}
	}
	return raw.Notify(appId, reuse_id, card.Icon, card.Summary, card.Body, actions, hints, 5)
}
