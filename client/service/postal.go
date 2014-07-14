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
	"encoding/json"
	"sync"

	"code.google.com/p/go-uuid/uuid"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/emblemcounter"
	"launchpad.net/ubuntu-push/bus/haptic"
	"launchpad.net/ubuntu-push/bus/notifications"
	"launchpad.net/ubuntu-push/bus/urldispatcher"
	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/messaging"
	"launchpad.net/ubuntu-push/nih"
	"launchpad.net/ubuntu-push/sounds"
	"launchpad.net/ubuntu-push/util"
)

type messageHandler func(*click.AppId, string, *launch_helper.HelperOutput) error

// PostalService is the dbus api
type PostalService struct {
	DBusService
	mbox              map[string][]string
	msgHandler        messageHandler
	HelperLauncher    launch_helper.HelperLauncher
	messagingMenu     *messaging.MessagingMenu
	EmblemCounterEndp bus.Endpoint
	HapticEndp        bus.Endpoint
	NotificationsEndp bus.Endpoint
	URLDispatcherEndp bus.Endpoint
	actionsCh         <-chan *notifications.RawAction
}

var (
	PostalServiceBusAddress = bus.Address{
		Interface: "com.ubuntu.Postal",
		Path:      "/com/ubuntu/Postal",
		Name:      "com.ubuntu.Postal",
	}
)

var (
	SystemUpdateUrl = "settings:///system/system-update"
)

// NewPostalService() builds a new service and returns it.
func NewPostalService(installedChecker click.InstalledChecker, log logger.Logger) *PostalService {
	var svc = &PostalService{}
	svc.Log = log
	svc.Bus = bus.SessionBus.Endpoint(PostalServiceBusAddress, log)
	svc.installedChecker = installedChecker
	svc.messagingMenu = messaging.New(log)
	svc.HelperLauncher = launch_helper.NewTrivialHelperLauncher(log)
	svc.NotificationsEndp = bus.SessionBus.Endpoint(notifications.BusAddress, log)
	svc.EmblemCounterEndp = bus.SessionBus.Endpoint(emblemcounter.BusAddress, log)
	svc.HapticEndp = bus.SessionBus.Endpoint(haptic.BusAddress, log)
	svc.URLDispatcherEndp = bus.SessionBus.Endpoint(urldispatcher.BusAddress, log)
	svc.msgHandler = svc.messageHandler
	return svc
}

// SetMessageHandler() sets the message-handling callback
func (svc *PostalService) SetMessageHandler(callback messageHandler) {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	svc.msgHandler = callback
}

// GetMessageHandler() returns the (possibly nil) messaging handler callback
func (svc *PostalService) GetMessageHandler() messageHandler {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	return svc.msgHandler
}

// Start() dials the bus, grab the name, and listens for method calls.
func (svc *PostalService) Start() error {
	err := svc.DBusService.Start(bus.DispatchMap{
		"PopAll": svc.notifications,
		"Post":   svc.inject,
	}, PostalServiceBusAddress)
	if err != nil {
		return err
	}
	err = svc.takeTheBus()
	if err != nil {
		return err
	}
	go svc.handleActions()
	return err
}

// handleClicks loops on the actions channel waiting for actions and handling them
func (svc *PostalService) handleActions() {
	svc.lock.RLock()
	ch := svc.actionsCh
	svc.lock.RUnlock()
	for action := range ch {
		if action == nil {
			svc.Log.Debugf("handleActions got nil action; ignoring")
			continue
		}
		url := action.Action
		// it doesn't get much simpler...
		urld := urldispatcher.New(svc.URLDispatcherEndp, svc.Log)
		// this ignores the error (it's been logged already)
		urld.DispatchURL(url)
	}
}

func (svc *PostalService) takeTheBus() error {
	if svc.Log == nil {
		return ErrNotConfigured
	}

	endps := []struct {
		name string
		endp bus.Endpoint
	}{
		{"notifications", svc.NotificationsEndp},
		{"emblemcounter", svc.EmblemCounterEndp},
		{"haptic", svc.HapticEndp},
		{"urldispatcher", svc.URLDispatcherEndp},
	}
	for _, endp := range endps {
		if endp.endp == nil {
			svc.Log.Errorf("endpoint for %s is nil", endp.name)
			return ErrNotConfigured
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(endps))
	for _, endp := range endps {
		go func(name string, endp bus.Endpoint) {
			util.NewAutoRedialer(endp).Redial()
			svc.Log.Debugf("%s dialed in", name)
			wg.Done()
		}(endp.name, endp.endp)
	}
	wg.Wait()
	actionsCh, err := notifications.Raw(svc.NotificationsEndp, svc.Log).WatchActions()
	if err == nil {
		svc.lock.Lock()
		svc.actionsCh = actionsCh
		svc.lock.Unlock()
	}

	return err
}

func (svc *PostalService) notifications(path string, args, _ []interface{}) ([]interface{}, error) {
	app, err := svc.grabDBusPackageAndAppId(path, args, 0)
	if err != nil {
		return nil, err
	}

	svc.lock.Lock()
	defer svc.lock.Unlock()

	if svc.mbox == nil {
		return []interface{}{[]string(nil)}, nil
	}
	msgs := svc.mbox[app.Original()]
	delete(svc.mbox, app.Original())

	return []interface{}{msgs}, nil
}

var newNid = uuid.New

func (svc *PostalService) inject(path string, args, _ []interface{}) ([]interface{}, error) {
	app, err := svc.grabDBusPackageAndAppId(path, args, 1)
	if err != nil {
		return nil, err
	}
	notif, ok := args[1].(string)
	if !ok {
		return nil, ErrBadArgType
	}

	nid := newNid()

	return nil, svc.Inject(app, nid, notif)
}

// Inject() signals to an application over dbus that a notification
// has arrived.
func (svc *PostalService) Inject(app *click.AppId, nid string, notif string) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.mbox == nil {
		svc.mbox = make(map[string][]string)
	}
	output := svc.HelperLauncher.Run(app, []byte(notif))
	appId := app.Original()
	// XXX also track the nid in the mbox
	svc.mbox[appId] = append(svc.mbox[appId], string(output.Message))

	if svc.msgHandler != nil {
		err := svc.msgHandler(app, nid, output)
		if err != nil {
			svc.DBusService.Log.Errorf("msgHandler returned %v", err)
			return err
		}
		svc.DBusService.Log.Debugf("call to msgHandler successful")
	}

	return svc.Bus.Signal("Post", "/"+string(nih.Quote([]byte(app.Package))), []interface{}{appId})
}

func (svc *PostalService) messageHandler(app *click.AppId, nid string, output *launch_helper.HelperOutput) error {
	svc.messagingMenu.Present(app, nid, output.Notification)
	nots := notifications.Raw(svc.NotificationsEndp, svc.Log)
	_, err := nots.Present(app, nid, output.Notification)
	emblemcounter.New(svc.EmblemCounterEndp, svc.Log).Present(app, nid, output.Notification)
	haptic.New(svc.HapticEndp, svc.Log).Present(app, nid, output.Notification)
	sounds.New(svc.Log).Present(app, nid, output.Notification)
	return err
}

func (svc *PostalService) InjectBroadcast() (uint32, error) {
	icon := "update_manager_icon"
	summary := "There's an updated system image."
	body := "Tap to open the system updater."
	actions := []string{"Switch to app"} // action value not visible on the phone
	card := &launch_helper.Card{Icon: icon, Summary: summary, Body: body, Actions: actions, Popup: true}
	helperOutput := &launch_helper.HelperOutput{[]byte(""), &launch_helper.Notification{Card: card}}
	jsonNotif, err := json.Marshal(helperOutput)
	if err != nil {
		// XXX: how can we test this branch?
		svc.Log.Errorf("Failed to marshal notification: %v - %v", helperOutput, err)
		return 0, err
	}
	appId, _ := click.ParseAppId("_ubuntu-push-client")
	return 0, svc.Inject(appId, SystemUpdateUrl, string(jsonNotif))
}
