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

// Package unitygreeter retrieves information about the Unity Greeter
// using Unity's dbus interface
package unitygreeter

import (
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
)

// Well known address for the UnityGreeter API
var BusAddress bus.Address = bus.Address{
	Interface: "com.canonical.UnityGreeter",
	Path:      "/",
	Name:      "com.canonical.UnityGreeter",
}

// UnityGreeter encapsulates info needed to call out to the UnityGreeter API
type UnityGreeter struct {
	bus bus.Endpoint
	log logger.Logger
}

// New returns a new UnityGreeter that'll use the provided bus.Endpoint
func New(endp bus.Endpoint, log logger.Logger) *UnityGreeter {
	return &UnityGreeter{endp, log}
}

// GetUnityGreeter returns the window stack state
func (greeter *UnityGreeter) IsActive() bool {
	result, err := greeter.bus.GetProperty("IsActive")
	if err != nil {
		greeter.log.Errorf("GetUnityGreeter call returned %v", err)
		return false
	}
	return result.(bool)
}
