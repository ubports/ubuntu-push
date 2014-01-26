package main

import (
	"launchpad.net/go-dbus/v1"
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/connectivity"
	"launchpad.net/ubuntu-push/bus/networkmanager"
	"launchpad.net/ubuntu-push/bus/notifications"
	"launchpad.net/ubuntu-push/bus/urldispatcher"
	"launchpad.net/ubuntu-push/client/session"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/util"
	"launchpad.net/ubuntu-push/whoopsie/identifier"
	"os"
)

type configuration struct {
	connectivity.ConnectivityConfig
	session.ClientConfig
}

const (
	configFName string = "client.json"
)

func notify_update(nots *notifications.RawNotifications, log logger.Logger) {
	action_id := "my_action_id"
	a := []string{action_id, "Go get it!"} // action value not visible on the phone
	h := map[string]*dbus.Variant{"x-canonical-switch-to-application": &dbus.Variant{true}}
	not_id, err := nots.Notify(
		"ubuntu-push-client",               // app name
		uint32(0),                          // id
		"update_manager_icon",              // icon
		"There's an updated system image!", // summary
		"You've got to get it! Now! Run!",  // body
		a,              // actions
		h,              // hints
		int32(10*1000), // timeout
	)
	if err != nil {
		log.Fatalf("%s", err)
	}
	log.Debugf("Got notification id %d\n", not_id)
}

func main() {
	log := logger.NewSimpleLogger(os.Stderr, "debug")
	f, err := os.Open(configFName)
	if err != nil {
		log.Fatalf("reading config: %v", err)
	}
	cfg := &configuration{}
	err = config.ReadConfig(f, cfg)
	if err != nil {
		log.Fatalf("reading config: %v", err)
	}
	whopId := identifier.New()
	err = whopId.Generate()
	if err != nil {
		log.Fatalf("Generating device id: %v", err)
	}
	deviceId := whopId.String()
	log.Debugf("Connecting as device id %s", deviceId)
	session, err := session.NewSession(cfg.ClientConfig, log, deviceId)
	if err != nil {
		log.Fatalf("%s", err)
	}
	// ^^ up to this line, things that never change
	var is_connected, ok bool
	// vv from this line, things that never stay the same
	for {
		log.Debugf("Here we go!")
		is_connected = false
		connCh := make(chan bool)
		iniCh := make(chan uint32)

		notEndp := bus.SessionBus.Endpoint(notifications.BusAddress, log)
		urlEndp := bus.SessionBus.Endpoint(urldispatcher.BusAddress, log)

		go func() { iniCh <- util.AutoRetry(session.Reset) }()
		go func() { iniCh <- util.AutoRedial(notEndp) }()
		go func() { iniCh <- util.AutoRedial(urlEndp) }()
		go connectivity.ConnectedState(bus.SystemBus.Endpoint(networkmanager.BusAddress, log), cfg.ConnectivityConfig, log, connCh)

		<-iniCh
		<-iniCh
		<-iniCh
		nots := notifications.Raw(notEndp, log)
		urld := urldispatcher.New(urlEndp, log)

		actnCh, err := nots.WatchActions()
		if err != nil {
			log.Errorf("%s", err)
			continue
		}

	InnerLoop:
		for {
			select {
			case is_connected, ok = <-connCh:
				// handle connectivty changes
				// disconnect session if offline, reconnect if online
				if !ok {
					log.Errorf("connectivity checker crashed? restarting everything")
					break InnerLoop
				}
				// fallthrough
				// oh, silly ol' go doesn't like that.
				if is_connected {
					err = session.Reset()
					if err != nil {
						break InnerLoop
					}
				}
			case <-session.ErrCh:
				// handle session errors
				// restart if online, otherwise ignore
				if is_connected {
					err = session.Reset()
					if err != nil {
						break InnerLoop
					}
				}
			case <-session.MsgCh:
				// handle push notifications
				// pop up client notification
				// what to do does not depend on the rest
				// (... for now)
				log.Debugf("got a notification! let's pop it up.")
				notify_update(nots, log)
			case <-actnCh:
				// handle action clicks
				// launch system updates
				// what to do does not depend on the rest
				// (... for now)
				urld.DispatchURL("settings:///system/system-update")
			}
		}
	}
}
