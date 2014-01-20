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
// Here we define the Bus itself.
package bus

import (
	"launchpad.net/go-dbus/v1"
	"launchpad.net/ubuntu-push/logger"
)

/*****************************************************************
 *    Bus (and its implementation)
 */

type Bus interface {
	String() string
	Connect(Address, logger.Logger) (Endpoint, error)
}

type concreteBus dbus.StandardBus

// ensure concreteBus implements Bus
var _ Bus = new(concreteBus)

// no constructor, just two standard, constant, busses
var SessionBus Bus = concreteBus(dbus.SessionBus)
var SystemBus  Bus = concreteBus(dbus.SystemBus)

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

// Connect() connects to the bus, and returns the bus endpoint (and/or error).
func (bus concreteBus) Connect(addr Address, log logger.Logger) (Endpoint, error) {
	conn, err := dbus.Connect(bus.dbusType())
	if err != nil {
		return nil, err
	} else {
		return &endpoint{conn, addr.Name, addr.Path, addr.Interface, log}, nil
	}
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

// Address is just a back of configuration
type Address struct {
	Name      string
	Path      string
	Interface string
}
