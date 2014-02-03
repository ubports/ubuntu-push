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

// The client package implements the Ubuntu Push Notifications client-side
// daemon.
package client

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
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

// ClientConfig holds the client configuration
type ClientConfig struct {
	connectivity.ConnectivityConfig // q.v.
	// A reasonably large maximum ping time
	ExchangeTimeout config.ConfigTimeDuration `json:"exchange_timeout"`
	// The server to connect to
	Addr config.ConfigHostPort
	// The PEM-encoded server certificate
	CertPEMFile string `json:"cert_pem_file"`
}

// Client is the Ubuntu Push Notifications client-side daemon.
type Client struct {
	config                ClientConfig
	log                   logger.Logger
	pem                   []byte
	idder                 identifier.Id
	deviceId              string
	notificationsEndp     bus.Endpoint
	urlDispatcherEndp     bus.Endpoint
	connectivityEndp      bus.Endpoint
	connCh                chan bool
	connState             bool
	actionsCh             <-chan notifications.RawActionReply
	session               *session.ClientSession
	sessionRetrierStopper chan bool
}

// Configure loads the configuration specified in configPath, and sets it up.
func (client *Client) Configure(configPath string) error {
	f, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("opening config: %v", err)
	}
	err = config.ReadConfig(f, &client.config)
	if err != nil {
		return fmt.Errorf("reading config: %v", err)
	}
	// later, we'll be specifying logging options in the config file
	client.log = logger.NewSimpleLogger(os.Stderr, "error")

	// overridden for testing
	client.idder = identifier.New()
	client.notificationsEndp = bus.SessionBus.Endpoint(notifications.BusAddress, client.log)
	client.urlDispatcherEndp = bus.SessionBus.Endpoint(urldispatcher.BusAddress, client.log)
	client.connectivityEndp = bus.SystemBus.Endpoint(networkmanager.BusAddress, client.log)

	client.connCh = make(chan bool)

	if client.config.CertPEMFile != "" {
		client.pem, err = ioutil.ReadFile(client.config.CertPEMFile)
		if err != nil {
			return fmt.Errorf("reading PEM file: %v", err)
		}
		// sanity check
		p, _ := pem.Decode(client.pem)
		if p == nil {
			return fmt.Errorf("no PEM found in PEM file")
		}
	}

	return nil
}

// getDeviceId gets the whoopsie identifier for the device
func (client *Client) getDeviceId() error {
	err := client.idder.Generate()
	if err != nil {
		return err
	}
	client.deviceId = client.idder.String()
	return nil
}

// takeTheBus starts the connection(s) to D-Bus and sets up associated event channels
func (client *Client) takeTheBus() error {
	go connectivity.ConnectedState(client.connectivityEndp,
		client.config.ConnectivityConfig, client.log, client.connCh)
	iniCh := make(chan uint32)
	go func() { iniCh <- util.AutoRedial(client.notificationsEndp) }()
	go func() { iniCh <- util.AutoRedial(client.urlDispatcherEndp) }()
	<-iniCh
	<-iniCh

	actionsCh, err := notifications.Raw(client.notificationsEndp, client.log).WatchActions()
	client.actionsCh = actionsCh
	return err
}

// handleConnState deals with connectivity events
func (client *Client) handleConnState(connState bool) {
	if client.connState == connState {
		// nothing to do!
		return
	}
	client.connState = connState
	if client.session == nil {
		sess, err := session.NewSession(string(client.config.Addr), client.pem,
			client.config.ExchangeTimeout.Duration, client.deviceId, client.log)
		if err != nil {
			panic("Don't know how to handle session creation failure.")
		}
		client.session = sess
	}
	if connState {
		// connected
		if client.sessionRetrierStopper != nil {
			client.sessionRetrierStopper <- true
			client.sessionRetrierStopper = nil
		}
		ar := &util.AutoRetrier{
			make(chan bool, 1),
			client.session.Dial,
			util.Jitter}
		client.sessionRetrierStopper = ar.Stop
		go ar.Retry()
	} else {
		// disconnected
		if client.sessionRetrierStopper != nil {
			client.sessionRetrierStopper <- true
			client.sessionRetrierStopper = nil
		} else {
			client.session.Close()
		}
	}
}

// handleNotification deals with receiving a notification
func (client *Client) handleNotification() error {
	action_id := "dummy_id"
	a := []string{action_id, "Go get it!"} // action value not visible on the phone
	h := map[string]*dbus.Variant{"x-canonical-switch-to-application": &dbus.Variant{true}}
	nots := notifications.Raw(client.notificationsEndp, client.log)
	not_id, err := nots.Notify(
		"ubuntu-push-client",               // app name
		uint32(0),                          // id
		"update_manager_icon",              // icon
		"There's an updated system image!", // summary
		"You've got to get it! Now! Run!",  // body
		a,              // actions
		h,              // hints
		int32(10*1000), // timeout (ms)
	)
	if err != nil {
		client.log.Errorf("showing notification: %s", err)
		return err
	}
	client.log.Debugf("got notification id %d", not_id)
	return nil
}

// handleClick deals with the user clicking a notification
func (client *Client) handleClick() error {
	// it doesn't get much simpler...
	urld := urldispatcher.New(client.urlDispatcherEndp, client.log)
	return urld.DispatchURL("settings:///system/system-update")
}
