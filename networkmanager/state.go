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

package networkmanager

type State uint32

// the NetworkManager states, as per
// https://wiki.gnome.org/Projects/NetworkManager/DBusInterface/LatestDBusAPI
const (
	Unknown State = iota * 10
	Asleep
	Disconnected
	Disconnecting
	Connecting
	ConnectedLocal
	ConnectedSite
	ConnectedGlobal
	_max_state
)

var names = map[State]string{
	Unknown:         "Unknown",
	Asleep:          "Asleep",
	Disconnected:    "Disconnected",
	Disconnecting:   "Disconnecting",
	Connecting:      "Connecting",
	ConnectedLocal:  "Connected Local",
	ConnectedSite:   "Connected Site",
	ConnectedGlobal: "Connected Global",
}

// give its state a descriptive stringification
func (state State) String() string {
	if s, ok := names[state]; ok {
		return s
	}
	return names[Unknown]
}
