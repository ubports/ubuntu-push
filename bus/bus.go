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

import (
	"launchpad.net/go-dbus/v1"
	"launchpad.net/ubuntu-push/bus/connection"
	"launchpad.net/ubuntu-push/logger"
)

type Interface interface {
	String() string
	Connect(info Info, log logger.Logger) (connection.Interface, error)
}

type Info struct {
	Name      string
	Path      string
	Interface string
}

type Bus bool

const (
	SessionBus Bus = false
	SystemBus  Bus = true
)

func (bt Bus) String() string {
	if bt {
		return "SystemBus"
	} else {
		return "SessionBus"
	}
}

func (bt Bus) dbusType() dbus.StandardBus {
	if bt {
		return dbus.SystemBus
	} else {
		return dbus.SessionBus
	}
}

// Connect() connects to the bus, and returns the bus.connection (and/or error).
func (bt Bus) Connect(info Info, log logger.Logger) (connection.Interface, error) {
	conn, err := dbus.Connect(bt.dbusType())
	if err != nil {
		return nil, err
	} else {
		return connection.New(conn, info.Name, info.Path, info.Interface, log), nil
	}
}

var _ Interface = Bus(true)
