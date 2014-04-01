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
	"errors"
	"fmt"
	"launchpad.net/go-dbus/v1"
	"launchpad.net/ubuntu-push/logger"
)

/*****************************************************************
 *    Endpoint (and its implementation)
 */

// bus.Endpoint represents the DBus connection itself.
type Endpoint interface {
	WatchSignal(member string, f func(...interface{}), d func()) error
	Call(member string, args ...interface{}) ([]interface{}, error)
	GetProperty(property string) (interface{}, error)
	Dial() error
	Close()
	String() string
}

type endpoint struct {
	busT  Bus
	bus   *dbus.Connection
	proxy *dbus.ObjectProxy
	addr  Address
	log   logger.Logger
}

// constructor
func newEndpoint(bus Bus, addr Address, log logger.Logger) *endpoint {
	return &endpoint{busT: bus, addr: addr, log: log}
}

// ensure endpoint implements Endpoint
var _ Endpoint = &endpoint{}

/*
   public methods
*/

// Dial() (re)establishes the connection with dbus
func (endp *endpoint) Dial() error {
	bus, err := dbus.Connect(endp.busT.(concreteBus).dbusType())
	if err != nil {
		return err
	}
	d := dbus.BusDaemon{bus.Object(dbus.BUS_DAEMON_NAME, dbus.BUS_DAEMON_PATH)}
	name := endp.addr.Name
	hasOwner, err := d.NameHasOwner(name)
	if err != nil {
		endp.log.Debugf("Unable to determine ownership of %#v: %v", name, err)
		bus.Close()
		return err
	}
	if !hasOwner {
		// maybe it's waiting to be activated?
		names, err := d.ListActivatableNames()
		if err != nil {
			endp.log.Debugf("%#v has no owner, and when listing activatable: %v", name, err)
			bus.Close()
			return err
		}
		found := false
		for _, name := range names {
			if name == name {
				found = true
				break
			}
		}
		if !found {
			msg := fmt.Sprintf("%#v has no owner, and not in activatables", name)
			endp.log.Debugf(msg)
			bus.Close()
			return errors.New(msg)
		}
	}
	endp.log.Infof("%#v dialed in.", name)
	endp.bus = bus
	endp.proxy = bus.Object(name, dbus.ObjectPath(endp.addr.Path))
	return nil
}

// WatchSignal() takes a member name, sets up a watch for it (on the name,
// path and interface provided when creating the endpoint), and then calls f()
// with the unpacked value. If it's unable to set up the watch it returns an
// error. If the watch fails once established, d() is called. Typically f()
// sends the values over a channel, and d() would close the channel.
func (endp *endpoint) WatchSignal(member string, f func(...interface{}), d func()) error {
	watch, err := endp.proxy.WatchSignal(endp.addr.Interface, member)
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
func (endp *endpoint) Call(member string, args ...interface{}) ([]interface{}, error) {
	msg, err := endp.proxy.Call(endp.addr.Interface, member, args...)
	if err != nil {
		return nil, err
	}
	rvs := endp.unpackOneMsg(msg, member)
	return rvs, nil
}

// GetProperty uses the org.freedesktop.DBus.Properties interface's Get method
// to read a given property on the name, path and interface provided when
// creating the endpoint. The return value is unpacked into a dbus.Variant,
// and its value returned.
func (endp *endpoint) GetProperty(property string) (interface{}, error) {
	msg, err := endp.proxy.Call("org.freedesktop.DBus.Properties", "Get", endp.addr.Interface, property)
	if err != nil {
		return nil, err
	}
	variantvs := endp.unpackOneMsg(msg, property)
	switch len(variantvs) {
	default:
		return nil, fmt.Errorf("Too many values in Properties.Get response: %d", len(variantvs))
	case 0:
		return nil, fmt.Errorf("Not enough values in Properties.Get response: %d", len(variantvs))
	case 1:
		// carry on
	}
	variant, ok := variantvs[0].(*dbus.Variant)
	if !ok {
		return nil, fmt.Errorf("Response from Properties.Get wasn't a *dbus.Variant")
	}
	return variant.Value, nil
}

// Close the connection to dbus.
func (endp *endpoint) Close() {
	if endp.bus != nil {
		endp.bus.Close()
		endp.bus = nil
		endp.proxy = nil
	}
}

// String() performs advanced endpoint stringification
func (endp *endpoint) String() string {
	return fmt.Sprintf("<Connection to %s %#v>", endp.bus, endp.addr)
}

/*
   private methods
*/

// unpackOneMsg unpacks the value from the response msg
func (endp *endpoint) unpackOneMsg(msg *dbus.Message, member string) []interface{} {
	var varmap map[string]dbus.Variant
	if err := msg.Args(&varmap); err != nil {
		return msg.AllArgs()
	}
	return []interface{}{varmap}
}

// unpackMessages unpacks the value from the watch
func (endp *endpoint) unpackMessages(watch *dbus.SignalWatch, f func(...interface{}), d func(), member string) {
	for {
		msg, ok := <-watch.C
		if !ok {
			break
		}
		f(endp.unpackOneMsg(msg, member)...)
	}
	endp.log.Errorf("Got not-OK from %s watch", member)
	d()
}
