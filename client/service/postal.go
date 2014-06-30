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

package service

import (
	"strings"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/nih"
)

// PostalService is the dbus api
type PostalService struct {
	DBusService
	mbox           map[string][]string
	msgHandler     func(*launch_helper.HelperOutput) error
	HelperLauncher launch_helper.HelperLauncher
}

var (
	PostalServiceBusAddress = bus.Address{
		Interface: "com.ubuntu.Postal",
		Path:      "/com/ubuntu/Postal/*",
		Name:      "com.ubuntu.Postal",
	}
)

// NewPostalService() builds a new service and returns it.
func NewPostalService(bus bus.Endpoint, log logger.Logger) *PostalService {
	var svc = &PostalService{}
	svc.Log = log
	svc.Bus = bus
	svc.HelperLauncher = launch_helper.NewTrivialHelperLauncher(log)
	return svc
}

// SetMessageHandler() sets the message-handling callback
func (svc *PostalService) SetMessageHandler(callback func(*launch_helper.HelperOutput) error) {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	svc.msgHandler = callback
}

// GetMessageHandler() returns the (possibly nil) messaging handler callback
func (svc *PostalService) GetMessageHandler() func(*launch_helper.HelperOutput) error {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	return svc.msgHandler
}

// Start() dials the bus, grab the name, and listens for method calls.
func (svc *PostalService) Start() error {
	return svc.DBusService.Start(bus.DispatchMap{
		"Notifications": svc.notifications,
		"Inject":        svc.inject,
	}, PostalServiceBusAddress)
}

func (svc *PostalService) notifications(path string, args, _ []interface{}) ([]interface{}, error) {
	if len(args) != 0 {
		return nil, BadArgCount
	}
	appname := string(nih.Unquote([]byte(path[strings.LastIndex(path, "/")+1:])))

	svc.lock.Lock()
	defer svc.lock.Unlock()

	if svc.mbox == nil {
		return []interface{}{[]string(nil)}, nil
	}
	msgs := svc.mbox[appname]
	delete(svc.mbox, appname)

	return []interface{}{msgs}, nil
}

func (svc *PostalService) inject(path string, args, _ []interface{}) ([]interface{}, error) {
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
func (svc *PostalService) Inject(appname string, notif string) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.mbox == nil {
		svc.mbox = make(map[string][]string)
	}
	output := svc.HelperLauncher.Run(appname, []byte(notif))
	svc.mbox[appname] = append(svc.mbox[appname], string(output.Message))
	if svc.msgHandler != nil {
		err := svc.msgHandler(output)
		if err != nil {
			svc.DBusService.Log.Errorf("msgHandler returned %v", err)
			return err
		}
		svc.DBusService.Log.Debugf("call to msgHandler successful")
	}
	return svc.Bus.Signal("Notification", []interface{}{appname})
}
