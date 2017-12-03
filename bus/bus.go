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

// Package bus provides a simplified (and more testable?) interface to DBus.
package bus

// Here we define the Bus itself

import (
	"launchpad.net/go-dbus/v1"
	"github.com/ubports/ubuntu-push/logger"
)

/*****************************************************************
 *    Bus (and its implementation)
 */

// This is the Bus itself.
type Bus interface {
	String() string
	Endpoint(Address, logger.Logger) Endpoint
}

type concreteBus dbus.StandardBus

// ensure concreteBus implements Bus
var _ Bus = new(concreteBus)

// no bus.Bus constructor, just two standard, constant, busses
var (
	SessionBus Bus = concreteBus(dbus.SessionBus)
	SystemBus  Bus = concreteBus(dbus.SystemBus)
)

/*
   public methods
*/

func (bus concreteBus) String() string {
	if bus == concreteBus(dbus.SystemBus) {
		return "SystemBus"
	} else {
		return "SessionBus"
	}
}

// Endpoint returns a bus endpoint.
func (bus concreteBus) Endpoint(addr Address, log logger.Logger) Endpoint {
	return newEndpoint(bus, addr, log)
}

// Args helps build arguments for endpoint Call().
func Args(args ...interface{}) []interface{} {
	return args
}

/*
   private methods
*/

func (bus concreteBus) dbusType() dbus.StandardBus {
	return dbus.StandardBus(bus)
}

/*****************************************************************
 *    Address
 */

// bus.Address is just a bag of configuration
type Address struct {
	Name      string
	Path      string
	Interface string
}

var BusDaemonAddress = Address{
	dbus.BUS_DAEMON_NAME,
	string(dbus.BUS_DAEMON_PATH),
	dbus.BUS_DAEMON_IFACE,
}
