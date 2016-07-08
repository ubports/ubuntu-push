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
	"launchpad.net/ubuntu-push/click/cnotificationsettings"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/sounds"
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
	App      *click.AppId `json:"app,omitempty"`
	Action   string       `json:"act,omitempty"`
	ActionId int          `json:"aid,omitempty"`
	Nid      string       `json:"nid,omitempty"`
	RawId    uint32       `json:"-"`
}

// a raw notification provides a low-level interface to the f.d.o. dbus
// notifications api
type RawNotifications struct {
	bus   bus.Endpoint
	log   logger.Logger
	sound sounds.Sound
}

// Raw returns a new RawNotifications that'll use the provided bus.Endpoint
func Raw(endp bus.Endpoint, log logger.Logger, sound sounds.Sound) *RawNotifications {
	return &RawNotifications{endp, log, sound}
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
	_, err := raw.bus.WatchSignal("ActionInvoked",
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
		raw.log.Debugf("failed to set up the watch: %s", err)
		return nil, err
	}
	return ch, nil
}

var canUseBubblesNotify = cnotificationsettings.CanUseBubblesNotify

// Present displays a given card.
//
// If card.Actions is empty it's a plain, noninteractive bubble notification.
// If card.Actions has 1 action, it's an interactive notification.
// If card.Actions has 2 actions, it will show as a snap decision.
// If it has more actions, who knows (good luck).
func (raw *RawNotifications) Present(app *click.AppId, nid string, notification *launch_helper.Notification) bool {
	if notification == nil {
		panic("please check notification is not nil before calling present")
	}

	if (!canUseBubblesNotify(app)) {
		raw.log.Debugf("[%s] bubbles disabled by user for this app.", nid)

		if raw.sound != nil {
			return raw.sound.Present(app, nid, notification)
		} else {
			return false
		}
	}

	card := notification.Card

	if card == nil || !card.Popup || card.Summary == "" {
		raw.log.Debugf("[%s] notification has no popup card: %#v", nid, card)
		return false
	}

	hints := make(map[string]*dbus.Variant)
	hints["x-canonical-secondary-icon"] = &dbus.Variant{app.SymbolicIcon()}

	if raw.sound != nil {
		soundFile := raw.sound.GetSound(app, nid, notification)
		if soundFile != "" {
			hints["sound-file"] = &dbus.Variant{soundFile}
			raw.log.Debugf("[%s] notification will play sound: %s", nid, soundFile)
		}
	}

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
			raw.log.Errorf("[%s] while marshaling %#v to json: %v", nid, action, err)
			return false
		}
		actions[2*i] = string(act)
		actions[2*i+1] = action
	}
	switch len(card.Actions) {
	case 0:
		// nothing
	default:
		raw.log.Errorf("[%s] don't know what to do with %d actions; ignoring the rest", nid, len(card.Actions))
		actions = actions[:2]
		fallthrough
	case 1:
		hints["x-canonical-switch-to-application"] = &dbus.Variant{"true"}
	}

	raw.log.Debugf("[%s] creating popup (or snap decision) for %s (summary: %s)", nid, app.Base(), card.Summary)

	_, err := raw.Notify(appId, 0, card.Icon, card.Summary, card.Body, actions, hints, 30*1000)

	if err != nil {
		raw.log.Errorf("[%s] call to Notify failed: %v", nid, err)
		return false
	}

	return true
}
