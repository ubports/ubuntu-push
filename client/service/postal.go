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

	"code.google.com/p/go-uuid/uuid"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/notifications"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/messaging"
	"launchpad.net/ubuntu-push/nih"
	"launchpad.net/ubuntu-push/util"
)

// PostalService is the dbus api
type PostalService struct {
	DBusService
	mbox              map[string][]string
	msgHandler        func(string, string, *launch_helper.HelperOutput) error
	HelperLauncher    launch_helper.HelperLauncher
	messagingMenu     *messaging.MessagingMenu
	notificationsEndp bus.Endpoint
}

var (
	PostalServiceBusAddress = bus.Address{
		Interface: "com.ubuntu.Postal",
		Path:      "/com/ubuntu/Postal",
		Name:      "com.ubuntu.Postal",
	}
)

var (
	SystemUpdateUrl  = "settings:///system/system-update"
	ACTION_ID_PREFIX = "ubuntu-push-client::"
	ACTION_ID_SUFFIX = "::0"
)

// NewPostalService() builds a new service and returns it.
func NewPostalService(busEndp bus.Endpoint, notificationsEndp bus.Endpoint, log logger.Logger) *PostalService {
	var svc = &PostalService{}
	svc.Log = log
	svc.Bus = busEndp
	svc.messagingMenu = messaging.New(log)
	svc.HelperLauncher = launch_helper.NewTrivialHelperLauncher(log)
	svc.notificationsEndp = notificationsEndp
	svc.msgHandler = svc.messageHandler
	return svc
}

// SetMessageHandler() sets the message-handling callback
func (svc *PostalService) SetMessageHandler(callback func(string, string, *launch_helper.HelperOutput) error) {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	svc.msgHandler = callback
}

// GetMessageHandler() returns the (possibly nil) messaging handler callback
func (svc *PostalService) GetMessageHandler() func(string, string, *launch_helper.HelperOutput) error {
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
	util.NewAutoRedialer(svc.notificationsEndp).Redial()
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

var newNid = uuid.New

func (svc *PostalService) inject(path string, args, _ []interface{}) ([]interface{}, error) {
	if len(args) != 1 {
		return nil, BadArgCount
	}
	notif, ok := args[0].(string)
	if !ok {
		return nil, BadArgType
	}
	appname := string(nih.Unquote([]byte(path[strings.LastIndex(path, "/")+1:])))

	nid := newNid()

	return nil, svc.Inject(appname, nid, notif)
}

// Inject() signals to an application over dbus that a notification
// has arrived.
func (svc *PostalService) Inject(appname string, nid string, notif string) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.mbox == nil {
		svc.mbox = make(map[string][]string)
	}
	output := svc.HelperLauncher.Run(appname, []byte(notif))
	// XXX also track the nid in the mbox
	svc.mbox[appname] = append(svc.mbox[appname], string(output.Message))

	if svc.msgHandler != nil {
		err := svc.msgHandler(appname, nid, output)
		if err != nil {
			svc.DBusService.Log.Errorf("msgHandler returned %v", err)
			return err
		}
		svc.DBusService.Log.Debugf("call to msgHandler successful")
	}

	return svc.Bus.Signal("Notification", "/"+string(nih.Quote([]byte(appname))), []interface{}{appname})
}

func (svc *PostalService) messageHandler(appname string, nid string, output *launch_helper.HelperOutput) error {
	svc.messagingMenu.Present(appname, nid, output.Notification)
	nots := notifications.Raw(svc.notificationsEndp, svc.Log)
	_, err := nots.Present(appname, nid, output.Notification)

	return err
}

func (svc *PostalService) InjectBroadcast() (uint32, error) {
	// XXX: Present force us to send the url as the notificationId
	icon := "update_manager_icon"
	summary := "There's an updated system image."
	body := "Tap to open the system updater."
	actions := []string{"Switch to app"} // action value not visible on the phone
	card := &launch_helper.Card{Icon: icon, Summary: summary, Body: body, Actions: actions, Popup: true}
	output := &launch_helper.HelperOutput{[]byte(""), &launch_helper.Notification{Card: card}}
	return 0, svc.msgHandler("ubuntu-push-client", SystemUpdateUrl, output)
}
