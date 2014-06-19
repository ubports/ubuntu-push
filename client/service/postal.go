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

// package service implements the dbus-level service with which client
// applications are expected to interact.
package service

import (
	"strings"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/nih"
)

// Postal is the dbus api
type Postal struct {
	DBusService
	mbox       map[string][]string
	msgHandler func([]byte) error
}

var (
	PostalBusAddress = bus.Address{
		Interface: "com.ubuntu.Postal",
		Path:      "/com/ubuntu/Postal/*",
		Name:      "com.ubuntu.Postal",
	}
)

// NewPostal() builds a new service and returns it.
func NewPostal(bus bus.Endpoint, log logger.Logger) *Postal {
	var svc = &Postal{}
	svc.Log = log
	svc.Bus = bus
	return svc
}

// SetMessageHandler() sets the message-handling callback
func (svc *Postal) SetMessageHandler(callback func([]byte) error) {
	svc.Lock.RLock()
	defer svc.Lock.RUnlock()
	svc.msgHandler = callback
}

// GetMessageHandler() returns the (possibly nil) messaging handler callback
func (svc *Postal) GetMessageHandler() func([]byte) error {
	svc.Lock.RLock()
	defer svc.Lock.RUnlock()
	return svc.msgHandler
}

// Start() dials the bus, grab the name, and listens for method calls.
func (svc *Postal) Start() error {
	return svc.DBusService.Start(bus.DispatchMap{
		"Notifications": svc.notifications,
		"Inject":        svc.inject,
	}, PostalBusAddress)
}

func (svc *Postal) notifications(path string, args, _ []interface{}) ([]interface{}, error) {
	if len(args) != 0 {
		return nil, BadArgCount
	}
	appname := string(nih.Unquote([]byte(path[strings.LastIndex(path, "/")+1:])))

	svc.Lock.Lock()
	defer svc.Lock.Unlock()

	if svc.mbox == nil {
		return []interface{}{[]string(nil)}, nil
	}
	msgs := svc.mbox[appname]
	delete(svc.mbox, appname)

	return []interface{}{msgs}, nil
}

func (svc *Postal) inject(path string, args, _ []interface{}) ([]interface{}, error) {
	if len(args) != 1 {
		return nil, BadArgCount
	}
	notif, ok := args[0].(string)
	if !ok {
		return nil, BadArgType
	}
	appname := string(nih.Unquote([]byte(path[strings.LastIndex(path, "/")+1:])))

	return nil, svc.Inject(appname, notif)
}

// Inject() signals to an application over dbus that a notification
// has arrived.
func (svc *Postal) Inject(appname string, notif string) error {
	svc.Lock.Lock()
	defer svc.Lock.Unlock()
	if svc.mbox == nil {
		svc.mbox = make(map[string][]string)
	}
	svc.mbox[appname] = append(svc.mbox[appname], notif)
	if svc.msgHandler != nil {
		err := svc.msgHandler([]byte(notif))
		if err != nil {
			svc.DBusService.Log.Errorf("msgHandler returned %v", err)
			return err
		}
		svc.DBusService.Log.Debugf("call to msgHandler successful")
	}

	return svc.Bus.Signal("Notification", []interface{}{appname})
}
