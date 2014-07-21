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
	"os"
	"sync"

	"code.google.com/p/go-uuid/uuid"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/emblemcounter"
	"launchpad.net/ubuntu-push/bus/haptic"
	"launchpad.net/ubuntu-push/bus/notifications"
	"launchpad.net/ubuntu-push/bus/urldispatcher"
	"launchpad.net/ubuntu-push/bus/windowstack"
	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/messaging"
	"launchpad.net/ubuntu-push/messaging/reply"
	"launchpad.net/ubuntu-push/nih"
	"launchpad.net/ubuntu-push/sounds"
	"launchpad.net/ubuntu-push/util"
)

type messageHandler func(*click.AppId, string, *launch_helper.HelperOutput) error

// PostalService is the dbus api
type PostalService struct {
	DBusService
	mbox          map[string]*mBox
	msgHandler    messageHandler
	launchers     map[string]launch_helper.HelperLauncher
	HelperPool    launch_helper.HelperPool
	messagingMenu *messaging.MessagingMenu
	// the endpoints are only exposed for testing from client
	// XXX: uncouple some more so this isn't necessary
	EmblemCounterEndp bus.Endpoint
	HapticEndp        bus.Endpoint
	NotificationsEndp bus.Endpoint
	URLDispatcherEndp bus.Endpoint
	WindowStackEndp   bus.Endpoint
	// presenters:
	emblemCounter *emblemcounter.EmblemCounter
	haptic        *haptic.Haptic
	notifications *notifications.RawNotifications
	sound         *sounds.Sound
	// the url dispatcher, used for stuff.
	urlDispatcher urldispatcher.URLDispatcher
	windowStack   *windowstack.WindowStack
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
	useTrivialHelper = os.Getenv("UBUNTU_PUSH_USE_TRIVIAL_HELPER") != ""
)

// NewPostalService() builds a new service and returns it.
func NewPostalService(installedChecker click.InstalledChecker, log logger.Logger) *PostalService {
	var svc = &PostalService{}
	svc.Log = log
	svc.Bus = bus.SessionBus.Endpoint(PostalServiceBusAddress, log)
	svc.installedChecker = installedChecker
	svc.NotificationsEndp = bus.SessionBus.Endpoint(notifications.BusAddress, log)
	svc.EmblemCounterEndp = bus.SessionBus.Endpoint(emblemcounter.BusAddress, log)
	svc.HapticEndp = bus.SessionBus.Endpoint(haptic.BusAddress, log)
	svc.URLDispatcherEndp = bus.SessionBus.Endpoint(urldispatcher.BusAddress, log)
	svc.WindowStackEndp = bus.SessionBus.Endpoint(windowstack.BusAddress, log)
	svc.msgHandler = svc.messageHandler
	svc.launchers = launch_helper.DefaultLaunchers(log)
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
		"PopAll": svc.popAll,
		"Post":   svc.post,
	}, PostalServiceBusAddress)
	if err != nil {
		return err
	}
	actionsCh, err := svc.takeTheBus()
	if err != nil {
		return err
	}
	svc.urlDispatcher = urldispatcher.New(svc.URLDispatcherEndp, svc.Log)
	svc.notifications = notifications.Raw(svc.NotificationsEndp, svc.Log)
	svc.emblemCounter = emblemcounter.New(svc.EmblemCounterEndp, svc.Log)
	svc.haptic = haptic.New(svc.HapticEndp, svc.Log)
	svc.sound = sounds.New(svc.Log)
	svc.messagingMenu = messaging.New(svc.Log)
	if useTrivialHelper {
		svc.HelperPool = launch_helper.NewTrivialHelperPool(svc.Log)
	} else {
		svc.HelperPool = launch_helper.NewHelperPool(svc.launchers, svc.Log)
	}
	svc.windowStack = windowstack.New(svc.WindowStackEndp, svc.Log)

	go svc.consumeHelperResults(svc.HelperPool.Start())
	go svc.handleActions(actionsCh, svc.messagingMenu.Ch)
	return nil
}

// xxx Stop() closing channels and helper launcher

// handleactions loops on the actions channels waiting for actions and handling them
func (svc *PostalService) handleActions(actionsCh <-chan *notifications.RawAction, mmuActionsCh <-chan *reply.MMActionReply) {
Handle:
	for {
		select {
		case action, ok := <-actionsCh:
			if !ok {
				break Handle
			}
			if action == nil {
				svc.Log.Debugf("handleActions got nil action; ignoring")
			} else {
				url := action.Action
				// this ignores the error (it's been logged already)
				svc.urlDispatcher.DispatchURL(url)
			}
		case mmuAction, ok := <-mmuActionsCh:
			if !ok {
				break Handle
			}
			if mmuAction == nil {
				svc.Log.Debugf("handleActions (MMU) got nil action; ignoring")
			} else {
				svc.Log.Debugf("handleActions (MMU) got: %v", mmuAction)
				url := mmuAction.Action
				// remove the notification from the messagingmenu map
				svc.messagingMenu.RemoveNotification(mmuAction.Notification)
				// this ignores the error (it's been logged already)
				svc.urlDispatcher.DispatchURL(url)
			}

		}
	}
}

func (svc *PostalService) takeTheBus() (<-chan *notifications.RawAction, error) {
	endps := []struct {
		name string
		endp bus.Endpoint
	}{
		{"notifications", svc.NotificationsEndp},
		{"emblemcounter", svc.EmblemCounterEndp},
		{"haptic", svc.HapticEndp},
		{"urldispatcher", svc.URLDispatcherEndp},
		{"windowstack", svc.WindowStackEndp},
	}
	for _, endp := range endps {
		if endp.endp == nil {
			svc.Log.Errorf("endpoint for %s is nil", endp.name)
			return nil, ErrNotConfigured
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

	return notifications.Raw(svc.NotificationsEndp, svc.Log).WatchActions()
}

func (svc *PostalService) popAll(path string, args, _ []interface{}) ([]interface{}, error) {
	app, err := svc.grabDBusPackageAndAppId(path, args, 0)
	if err != nil {
		return nil, err
	}

	svc.lock.Lock()
	defer svc.lock.Unlock()

	if svc.mbox == nil {
		return []interface{}{[]string(nil)}, nil
	}
	appId := app.Original()
	box, ok := svc.mbox[appId]
	if !ok {
		return []interface{}{[]string(nil)}, nil
	}
	msgs := box.AllMessages()
	delete(svc.mbox, appId)

	return []interface{}{msgs}, nil
}

var newNid = uuid.New

func (svc *PostalService) post(path string, args, _ []interface{}) ([]interface{}, error) {
	app, err := svc.grabDBusPackageAndAppId(path, args, 1)
	if err != nil {
		return nil, err
	}
	notif, ok := args[1].(string)
	if !ok {
		return nil, ErrBadArgType
	}
	var dummy interface{}
	rawJSON := json.RawMessage(notif)
	err = json.Unmarshal(rawJSON, &dummy)
	if err != nil {
		return nil, ErrBadJSON
	}

	nid := newNid()

	return nil, svc.Post(app, nid, rawJSON)
}

// Post() signals to an application over dbus that a notification
// has arrived.
func (svc *PostalService) Post(app *click.AppId, nid string, payload json.RawMessage) error {
	arg := launch_helper.HelperInput{
		App:            app,
		NotificationId: nid,
		Payload:        payload,
	}
	var kind string
	if app.Click {
		kind = "click"
	} else {
		kind = "legacy"
	}
	svc.HelperPool.Run(kind, &arg)
	return nil
}

func (svc *PostalService) consumeHelperResults(ch chan *launch_helper.HelperResult) {
	for res := range ch {
		svc.handleHelperResult(res)
	}
}

func (svc *PostalService) handleHelperResult(res *launch_helper.HelperResult) {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.mbox == nil {
		svc.mbox = make(map[string]*mBox)
	}

	app := res.Input.App
	nid := res.Input.NotificationId
	output := res.HelperOutput

	appId := app.Original()
	box, ok := svc.mbox[appId]
	if !ok {
		box = new(mBox)
		svc.mbox[appId] = box
	}
	box.Append(output.Message, nid)

	if svc.msgHandler != nil {
		err := svc.msgHandler(app, nid, &output)
		if err != nil {
			svc.DBusService.Log.Errorf("msgHandler returned %v", err)
			return
		}
		svc.DBusService.Log.Debugf("call to msgHandler successful")
	}

	svc.Bus.Signal("Post", "/"+string(nih.Quote([]byte(app.Package))), []interface{}{appId})
}

func (svc *PostalService) messageHandler(app *click.AppId, nid string, output *launch_helper.HelperOutput) error {
	if !svc.windowStack.IsAppFocused(app) {
		svc.messagingMenu.Present(app, nid, output.Notification)
		_, err := svc.notifications.Present(app, nid, output.Notification)
		svc.emblemCounter.Present(app, nid, output.Notification)
		svc.haptic.Present(app, nid, output.Notification)
		svc.sound.Present(app, nid, output.Notification)
		return err
	} else {
		svc.Log.Debugf("Notification skipped because app is focused.")
		return nil
	}
}
