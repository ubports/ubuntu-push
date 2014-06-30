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
	"errors"
	"strings"

	"launchpad.net/go-dbus/v1"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/notifications"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/nih"
	"launchpad.net/ubuntu-push/util"
)

// PostalService is the dbus api
type PostalService struct {
	DBusService
	mbox              map[string][]string
	msgHandler        func(*launch_helper.HelperOutput) error
	HelperLauncher    launch_helper.HelperLauncher
	notificationsEndp bus.Endpoint
}

var (
	PostalServiceBusAddress = bus.Address{
		Interface: "com.ubuntu.Postal",
		Path:      "/com/ubuntu/Postal/*",
		Name:      "com.ubuntu.Postal",
	}
)

var (
	SystemUpdateUrl     = "settings:///system/system-update"
	ACTION_ID_SNOWFLAKE = "::ubuntu-push-client::"
	ACTION_ID_BROADCAST = ACTION_ID_SNOWFLAKE + SystemUpdateUrl
)

// NewPostalService() builds a new service and returns it.
func NewPostalService(busEndp bus.Endpoint, notificationsEndp bus.Endpoint, log logger.Logger) *PostalService {
	var svc = &PostalService{}
	svc.Log = log
	svc.Bus = busEndp
	svc.HelperLauncher = launch_helper.NewTrivialHelperLauncher(log)
	svc.notificationsEndp = notificationsEndp
	svc.msgHandler = svc.messageHandler
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

func (svc *PostalService) TakeTheBus() (<-chan notifications.RawActionReply, error) {
	iniCh := make(chan uint32)
	go func() { iniCh <- util.NewAutoRedialer(svc.notificationsEndp).Redial() }()
	<-iniCh
	actionsCh, err := notifications.Raw(svc.notificationsEndp, svc.Log).WatchActions()
	return actionsCh, err
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

func (svc *PostalService) messageHandler(notif *launch_helper.HelperOutput) error {
	if notif.Notification == nil {
		svc.Log.Errorf("Ignoring message: notification is nil: %v", notif)
		return errors.New("Notification is nil.")
	} else if notif.Notification.Card != nil {
		card := notif.Notification.Card
		action := ""
		if len(card.Actions) >= 1 {
			action = card.Actions[0]
		}
		not_id, err := svc.SendNotification(
			ACTION_ID_SNOWFLAKE+action,
			card.Icon, card.Summary, card.Body)
		if err != nil {
			svc.Log.Errorf("showing notification: %s", err)
			return err
		}
		svc.Log.Debugf("got notification id %d", not_id)
	} else {
		svc.Log.Errorf("Ignoring message: notification.Card is nil: %v", notif)
		return errors.New("Notification.Card is nil.")
	}
	return nil
}

func (svc *PostalService) SendNotification(action_id, icon, summary, body string) (uint32, error) {
	a := []string{action_id, "Switch to app"} // action value not visible on the phone
	h := map[string]*dbus.Variant{"x-canonical-switch-to-application": &dbus.Variant{true}}
	nots := notifications.Raw(svc.notificationsEndp, svc.Log)
	return nots.Notify(
		"ubuntu-push-client", // app name
		uint32(0),            // id
		icon,                 // icon
		summary,              // summary
		body,                 // body
		a,                    // actions
		h,                    // hints
		int32(10*1000),       // timeout (ms)
	)
}
