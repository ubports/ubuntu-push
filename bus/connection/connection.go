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

// Package bus/connection provides a simplified (and more testable?)
// interface to DBus connections.
package connection

import (
	"launchpad.net/go-dbus/v1"
	"launchpad.net/ubuntu-push/logger"
)

type Interface interface {
	WatchSignal(member string, f func(interface{}), d func()) error
	Call(member string, args ...interface{}) (interface{}, error)
	Close()
}

type Connection struct {
	bus   *dbus.Connection
	name  string
	path  string
	iface string
	log   logger.Logger
}

// ensure Connection implements Interface
var _ Interface = Connection{}

// Constructor.
//
// NOTE the actual dbus connection is passed in; connection doesn't *actually*
// connect.
func New(bus *dbus.Connection, name, path, iface string, log logger.Logger) *Connection {
	return &Connection{bus, name, path, iface, log}
}

// unpackOneMsg unpacks the value from the response msg
func (conn Connection) unpackOneMsg(msg *dbus.Message, member string) (interface{}, error) {
	var v interface{}
	if err := msg.Args(&v); err != nil {
		conn.log.Errorf("Decoding %s: %s", member, err)
		return 0, err
	} else {
		return v, nil
	}
}

// unpackMessages unpacks the value from the watch
func (conn Connection) unpackMessages(watch *dbus.SignalWatch, f func(interface{}), d func(), member string) {
	for {
		msg, ok := <-watch.C
		if !ok {
			break
		}
		if val, err := conn.unpackOneMsg(msg, member); err == nil {
			// errors are ignored at this level
			f(val)
		}
	}
	conn.log.Errorf("Got not-OK from %s watch", member)
	d()
}

// WatchSignal() takes a member name and sets up a watch for it (on the name,
// path and interface provided when creating the bus), and then calls f() with
// the unpacked value. If it's unable to set up the watch it'll return an
// error. If the watch fails once established, d() is called. Typically f()
// sends the values over a channel, and d() would close the channel.
func (conn Connection) WatchSignal(member string, f func(interface{}), d func()) error {
	watch, err := conn.bus.WatchSignal(&dbus.MatchRule{
		Type:      dbus.TypeSignal,
		Sender:    conn.name,
		Path:      dbus.ObjectPath(conn.path),
		Interface: conn.iface,
		Member:    member,
	})
	if err != nil {
		conn.log.Debugf("Failed to set up the watch: %s", err)
		return err
	}

	go conn.unpackMessages(watch, f, d, member)

	return nil
}

// Call() invokes the provided member method (on the name, path and interface
// provided when creating the bus). The return value is unpacked before being
// set on.
func (conn Connection) Call(member string, args ...interface{}) (interface{}, error) {
	proxy := conn.bus.Object(conn.name, dbus.ObjectPath(conn.path))
	if msg, err := proxy.Call(conn.iface, member, args...); err == nil {
		return conn.unpackOneMsg(msg, member)
	} else {
		return 0, err
	}
}

// Close the connection to the bus.
func (conn Connection) Close() {
	conn.bus.Close()
}
