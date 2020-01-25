/*
 Copyright 2015 Canonical Ltd.

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

// Package urfkill wraps a couple of URfkill's DBus API points to
// watch for flight mode state changes.
package urfkill

import (
	"launchpad.net/go-dbus"

	"github.com/ubports/ubuntu-push/bus"
	"github.com/ubports/ubuntu-push/logger"
)

// URfkill lives on a well-knwon bus.Address
var BusAddress bus.Address = bus.Address{
	Interface: "org.freedesktop.URfkill",
	Path:      "/org/freedesktop/URfkill",
	Name:      "org.freedesktop.URfkill",
}

// URfkill lives on a well-knwon bus.Address
var WLANKillswitchBusAddress bus.Address = bus.Address{
	Interface: "org.freedesktop.URfkill.Killswitch",
	Path:      "/org/freedesktop/URfkill/WLAN",
	Name:      "org.freedesktop.URfkill",
}

/*****************************************************************
 *    URfkill (and its implementation)
 */

type KillswitchState int32

const (
	KillswitchStateUnblocked   KillswitchState = 0
	KillswitchStateSoftBlocked KillswitchState = 1
	KillswitchStateHardBlocked KillswitchState = 2
)

type URfkill interface {
	// IsFlightMode returns flight mode state.
	IsFlightMode() bool
	// WatchFlightMode listens for changes to URfkill's flight
	// mode state, and sends them out over the channel returned.
	WatchFlightMode() (<-chan bool, bus.Cancellable, error)
	// GetWLANKillswitchState fetches and returns URfkill's
	// WLAN killswitch state.
	GetWLANKillswitchState() KillswitchState
	// WatchWLANKillswitchState listens for changes of URfkill's
	// WLAN killswtich state, and sends them out over the channel returned.
	WatchWLANKillswitchState() (<-chan KillswitchState, bus.Cancellable, error)
}

type uRfkill struct {
	bus            bus.Endpoint
	wlanKillswitch bus.Endpoint
	log            logger.Logger
}

// New returns a new URfkill that'll use the provided bus.Endpoints
// for BusAddress and WLANKillswitchBusAddress
func New(endp bus.Endpoint, wlanKillswitch bus.Endpoint, log logger.Logger) URfkill {
	return &uRfkill{endp, wlanKillswitch, log}
}

// ensure uRfkill implements URfkill
var _ URfkill = &uRfkill{}

/*
   public methods
*/

func (ur *uRfkill) IsFlightMode() bool {
	var res bool
	err := ur.bus.Call("IsFlightMode", bus.Args(), &res)
	if err != nil {
		ur.log.Errorf("failed getting flight-mode state: %s", err)
		ur.log.Debugf("defaulting flight-mode state to false")
		return false
	}
	return res
}

func (ur *uRfkill) WatchFlightMode() (<-chan bool, bus.Cancellable, error) {
	ch := make(chan bool)
	w, err := ur.bus.WatchSignal("FlightModeChanged",
		func(ns ...interface{}) {
			stbool, ok := ns[0].(bool)
			if !ok {
				ur.log.Errorf("got weird flight-mode state: %#v", ns[0])
				return
			}
			ur.log.Debugf("got flight-mode change: %v", stbool)
			ch <- stbool
		},
		func() { close(ch) })
	if err != nil {
		ur.log.Debugf("Failed to set up the watch: %s", err)
		return nil, nil, err
	}

	return ch, w, nil
}

func (ur *uRfkill) GetWLANKillswitchState() KillswitchState {
	got, err := ur.wlanKillswitch.GetProperty("state")
	if err != nil {
		ur.log.Errorf("failed getting WLANKillswitchState: %s", err)
		ur.log.Debugf("defaulting WLANKillswitchState to true")
		return KillswitchStateUnblocked
	}

	v, ok := got.(int32)
	if !ok {
		ur.log.Errorf("got weird WLANKillswitchState: %#v", got)
		return KillswitchStateUnblocked
	}

	return KillswitchState(v)
}

func (ur *uRfkill) WatchWLANKillswitchState() (<-chan KillswitchState, bus.Cancellable, error) {
	ch := make(chan KillswitchState)
	w, err := ur.wlanKillswitch.WatchProperties(
		func(changed map[string]dbus.Variant, invalidated []string) {
			v, ok := changed["state"]
			if !ok {
				return
			}
			st, ok := v.Value.(int32)
			if !ok {
				ur.log.Errorf("got weird WLANKillswitchState via PropertiesChanged: %#v", v)
				return
			}
			ur.log.Debugf("got WLANKillswitchState change: %v", st)
			ch <- KillswitchState(st)
		}, func() { close(ch) })
	if err != nil {
		ur.log.Debugf("failed to set up the watch: %s", err)
		return nil, nil, err
	}

	return ch, w, nil
}
