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

// Package connectivity/networkmanager wraps a couple of networkmanager API
// points: the org.freedesktop.NetworkManager.state call, and listening for
// the StateChange signal.
package networkmanager

import (
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/connection"
	"launchpad.net/ubuntu-push/logger"
)

var BusInfo bus.Info = bus.Info{
	Interface: "org.freedesktop.NetworkManager",
	Path:      "/org/freedesktop/NetworkManager",
	Name:      "org.freedesktop.NetworkManager",
}

type NetworkManager struct {
	bus connection.Interface
	log logger.Logger
}

func New(conn connection.Interface, log logger.Logger) NetworkManager {
	return NetworkManager{conn, log}
}

// GetState fetches and returns NetworkManager's current state
func (nm NetworkManager) GetState() State {
	s, err := nm.bus.Call("state")

	if err != nil {
		nm.log.Errorf("Failed gettting current state: %s", err)
		nm.log.Debugf("Defaulting state to Unknown")
		return Unknown
	}

	return State(s.(uint32))
}

// WatchState listens for changes to NetworkManager's state, and sends
// them out over the channel returned.
func (nm NetworkManager) WatchState() (<-chan State, error) {
	ch := make(chan State)
	err := nm.bus.WatchSignal("StateChanged",
		func(n interface{}) { ch <- State(n.(uint32)) },
		func() { close(ch) })
	if err != nil {
		nm.log.Debugf("Failed to set up the watch: %s", err)
		return nil, err
	}

	return ch, nil
}
