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
	//"launchpad.net/go-dbus/v1"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
)

// URfkill lives on a well-knwon bus.Address
var BusAddress bus.Address = bus.Address{
	Interface: "org.freedesktop.URfkill",
	Path:      "/org/freedesktop/URfkill",
	Name:      "org.freedesktop.URfkill",
}

/*****************************************************************
 *    URfkill (and its implementation)
 */

type URfkill interface {
	// IsFlightMode returns flight mode state.
	IsFlightMode() bool
	// WatchFlightMode listens for changes to URfkill's flight
	// mode state, and sends them out over the channel returned.
	WatchFlightMode() (<-chan bool, bus.Cancellable, error)
}

type uRfkill struct {
	bus bus.Endpoint
	log logger.Logger
}

// New returns a new URfkill that'll use the provided bus.Endpoint
func New(endp bus.Endpoint, log logger.Logger) URfkill {
	return &uRfkill{endp, log}
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
