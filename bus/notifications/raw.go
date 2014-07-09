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
	"encoding/json"
	"errors"

	"launchpad.net/go-dbus/v1"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/click"
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
type RawAction struct {
	App      *click.AppId `json:"p"`
	ActionId int          `json:"i"`
	Nid      string       `json:"n"`
	Action   string       `json:"a"`
	RawId    uint32       `json:"r"`
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
func (raw *RawNotifications) WatchActions() (<-chan *RawAction, error) {
	ch := make(chan *RawAction)
	err := raw.bus.WatchSignal("ActionInvoked",
		func(ns ...interface{}) {
			if len(ns) != 2 {
				raw.log.Debugf("ActionInvoked delivered %d things instead of 2", len(ns))
				return
			}
			rawId, ok := ns[0].(uint32)
			if !ok {
				raw.log.Debugf("ActionInvoked's 1st param not a uint32")
				return
			}
			encodedAction, ok := ns[1].(string)
			if !ok {
				raw.log.Debugf("ActionInvoked's 2nd param not a string")
				return
			}
			var action *RawAction
			err := json.Unmarshal([]byte(encodedAction), &action)
			if err != nil {
				raw.log.Debugf("ActionInvoked's 2nd param not a json-encoded RawAction")
				return
			}
			action.RawId = rawId
			ch <- action
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
func (raw *RawNotifications) Present(app *click.AppId, nid string, notification *launch_helper.Notification) (uint32, error) {
	if notification == nil || notification.Card == nil || !notification.Card.Popup || notification.Card.Summary == "" {
		raw.log.Debugf("skipping notification: nil, or nil card, or not popup, or no summary: %#v", notification)
		return 0, nil
	}

	card := notification.Card

	hints := make(map[string]*dbus.Variant)
	hints["x-canonical-secondary-icon"] = &dbus.Variant{app.Icon()}

	appId := app.Original()
	actions := make([]string, 2*len(card.Actions))
	for i, action := range card.Actions {
		act, err := json.Marshal(&RawAction{
			App:      app,
			Nid:      nid,
			ActionId: i,
			Action:   action,
		})
		if err != nil {
			return 0, err
		}
		actions[2*i] = string(act)
		actions[2*i+1] = action
	}
	switch len(card.Actions) {
	case 1:
		hints["x-canonical-switch-to-application"] = &dbus.Variant{"true"}
	case 2:
		hints["x-canonical-snap-decisions"] = &dbus.Variant{"true"}
		hints["x-canonical-private-button-tint"] = &dbus.Variant{"true"}
		hints["x-canonical-non-shaped-icon"] = &dbus.Variant{"true"}
	}
	return raw.Notify(appId, 0, card.Icon, card.Summary, card.Body, actions, hints, 30*1000)
}
