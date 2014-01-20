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

package bus

// Here we define the Endpoint, which represents the DBus connection itself.

import (
	"launchpad.net/go-dbus/v1"
	"launchpad.net/ubuntu-push/logger"
)

/*****************************************************************
 *    Endpoint (and its implementation)
 */

// bus.Endpoint represents the DBus connection itself.
type Endpoint interface {
	WatchSignal(member string, f func(interface{}), d func()) error
	Call(member string, args ...interface{}) (interface{}, error)
	Close()
}

type endpoint struct {
	bus   *dbus.Connection
	name  string
	path  string
	iface string
	log   logger.Logger
}

// ensure endpoint implements Endpoint
var _ Endpoint = endpoint{}

/*
    public methods
 */

// WatchSignal() takes a member name and sets up a watch for it (on the name,
// path and interface provided when creating the endpoint), and then calls f()
// with the unpacked value. If it's unable to set up the watch it'll return an
// error. If the watch fails once established, d() is called. Typically f()
// sends the values over a channel, and d() would close the channel.
func (endp endpoint) WatchSignal(member string, f func(interface{}), d func()) error {
	watch, err := endp.bus.WatchSignal(&dbus.MatchRule{
		Type:      dbus.TypeSignal,
		Sender:    endp.name,
		Path:      dbus.ObjectPath(endp.path),
		Interface: endp.iface,
		Member:    member,
	})
	if err != nil {
		endp.log.Debugf("Failed to set up the watch: %s", err)
		return err
	}

	go endp.unpackMessages(watch, f, d, member)

	return nil
}

// Call() invokes the provided member method (on the name, path and interface
// provided when creating the endpoint). The return value is unpacked before
// being returned.
func (endp endpoint) Call(member string, args ...interface{}) (interface{}, error) {
	proxy := endp.bus.Object(endp.name, dbus.ObjectPath(endp.path))
	if msg, err := proxy.Call(endp.iface, member, args...); err == nil {
		return endp.unpackOneMsg(msg, member)
	} else {
		return 0, err
	}
}

// Close the connection to dbus.
func (endp endpoint) Close() {
	endp.bus.Close()
}


/*
    private methods
 */

// unpackOneMsg unpacks the value from the response msg
func (endp endpoint) unpackOneMsg(msg *dbus.Message, member string) (interface{}, error) {
	var v interface{}
	if err := msg.Args(&v); err != nil {
		endp.log.Errorf("Decoding %s: %s", member, err)
		return 0, err
	} else {
		return v, nil
	}
}

// unpackMessages unpacks the value from the watch
func (endp endpoint) unpackMessages(watch *dbus.SignalWatch, f func(interface{}), d func(), member string) {
	for {
		msg, ok := <-watch.C
		if !ok {
			break
		}
		if val, err := endp.unpackOneMsg(msg, member); err == nil {
			// errors are ignored at this level
			f(val)
		}
	}
	endp.log.Errorf("Got not-OK from %s watch", member)
	d()
}
