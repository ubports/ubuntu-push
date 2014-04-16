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

// Package client implements the Ubuntu Push Notifications client-side
// daemon.
package client

import (
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gopkg.in/qml.v0"
	"launchpad.net/go-dbus/v1"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/connectivity"
	"launchpad.net/ubuntu-push/bus/networkmanager"
	"launchpad.net/ubuntu-push/bus/notifications"
	"launchpad.net/ubuntu-push/bus/systemimage"
	"launchpad.net/ubuntu-push/bus/urldispatcher"
	"launchpad.net/ubuntu-push/client/session"
	"launchpad.net/ubuntu-push/client/session/levelmap"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/util"
	"launchpad.net/ubuntu-push/whoopsie/identifier"
)

var (
	getAuthorization = util.GetAuthorization
	shouldGetAuth    = false
)

// ClientConfig holds the client configuration
type ClientConfig struct {
	connectivity.ConnectivityConfig // q.v.
	// A reasonably large timeout for receive/answer pairs
	ExchangeTimeout config.ConfigTimeDuration `json:"exchange_timeout"`
	// A timeout to use when trying to connect to the server
	ConnectTimeout config.ConfigTimeDuration `json:"connect_timeout"`
	// The server to connect to or url to query for hosts to connect to
	Addr string
	// Host list management
	HostsCachingExpiryTime config.ConfigTimeDuration `json:"hosts_cache_expiry"`  // potentially refresh host list after
	ExpectAllRepairedTime  config.ConfigTimeDuration `json:"expect_all_repaired"` // worth retrying all servers after
	// The PEM-encoded server certificate
	CertPEMFile string `json:"cert_pem_file"`
	// The logging level (one of "debug", "info", "error")
	LogLevel logger.ConfigLogLevel `json:"log_level"`
}

// PushClient is the Ubuntu Push Notifications client-side daemon.
type PushClient struct {
	leveldbPath        string
	configPath         string
	config             ClientConfig
	log                logger.Logger
	pem                []byte
	idder              identifier.Id
	deviceId           string
	notificationsEndp  bus.Endpoint
	urlDispatcherEndp  bus.Endpoint
	connectivityEndp   bus.Endpoint
	systemImageEndp    bus.Endpoint
	systemImageInfo    *systemimage.InfoResult
	connCh             chan bool
	hasConnectivity    bool
	actionsCh          <-chan notifications.RawActionReply
	session            *session.ClientSession
	sessionConnectedCh chan uint32
	auth               string
}

var ACTION_ID_SNOWFLAKE = "::ubuntu-push-client::"

// Creates a new Ubuntu Push Notifications client-side daemon that will use
// the given configuration file.
func NewPushClient(configPath string, leveldbPath string) *PushClient {
	client := new(PushClient)
	client.configPath = configPath
	client.leveldbPath = leveldbPath

	return client
}

// configure loads its configuration, and sets it up.
func (client *PushClient) configure() error {
	_, err := os.Stat(client.configPath)
	if err != nil {
		return fmt.Errorf("config: %v", err)
	}
	err = config.ReadFiles(&client.config, client.configPath, "<flags>")
	if err != nil {
		return fmt.Errorf("config: %v", err)
	}
	// ignore spaces
	client.config.Addr = strings.Replace(client.config.Addr, " ", "", -1)
	if client.config.Addr == "" {
		return errors.New("no hosts specified")
	}

	// later, we'll be specifying more logging options in the config file
	client.log = logger.NewSimpleLogger(os.Stderr, client.config.LogLevel.Level())
	qml.SetLogger(client.log)

	// grab the authorization string from the accounts
	// TODO: remove this condition when we have a way to deal with failing authorizations
	if shouldGetAuth {
		auth, err := getAuthorization()
		if err != nil {
			return fmt.Errorf("unable to get the authorization token from the account: %v", err)
		}
		client.auth = auth
	}

	// overridden for testing
	client.idder = identifier.New()
	client.notificationsEndp = bus.SessionBus.Endpoint(notifications.BusAddress, client.log)
	client.urlDispatcherEndp = bus.SessionBus.Endpoint(urldispatcher.BusAddress, client.log)
	client.connectivityEndp = bus.SystemBus.Endpoint(networkmanager.BusAddress, client.log)
	client.systemImageEndp = bus.SystemBus.Endpoint(systemimage.BusAddress, client.log)

	client.connCh = make(chan bool, 1)
	client.sessionConnectedCh = make(chan uint32, 1)

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

// deriveSessionConfig dervies the session configuration from the client configuration bits.
func (client *PushClient) deriveSessionConfig(info map[string]interface{}) session.ClientSessionConfig {
	return session.ClientSessionConfig{
		ConnectTimeout:         client.config.ConnectTimeout.TimeDuration(),
		ExchangeTimeout:        client.config.ExchangeTimeout.TimeDuration(),
		HostsCachingExpiryTime: client.config.HostsCachingExpiryTime.TimeDuration(),
		ExpectAllRepairedTime:  client.config.ExpectAllRepairedTime.TimeDuration(),
		PEM:           client.pem,
		Info:          info,
		Authorization: client.auth,
	}
}

// getDeviceId gets the whoopsie identifier for the device
func (client *PushClient) getDeviceId() error {
	err := client.idder.Generate()
	if err != nil {
		return err
	}
	client.deviceId = client.idder.String()
	return nil
}

// takeTheBus starts the connection(s) to D-Bus and sets up associated event channels
func (client *PushClient) takeTheBus() error {
	go connectivity.ConnectedState(client.connectivityEndp,
		client.config.ConnectivityConfig, client.log, client.connCh)
	iniCh := make(chan uint32)
	go func() { iniCh <- util.NewAutoRedialer(client.notificationsEndp).Redial() }()
	go func() { iniCh <- util.NewAutoRedialer(client.urlDispatcherEndp).Redial() }()
	go func() { iniCh <- util.NewAutoRedialer(client.systemImageEndp).Redial() }()
	<-iniCh
	<-iniCh
	<-iniCh

	sysimg := systemimage.New(client.systemImageEndp, client.log)
	info, err := sysimg.Info()
	if err != nil {
		return err
	}
	client.systemImageInfo = info

	actionsCh, err := notifications.Raw(client.notificationsEndp, client.log).WatchActions()
	client.actionsCh = actionsCh
	return err
}

// initSession creates the session object
func (client *PushClient) initSession() error {
	info := map[string]interface{}{
		"device":       client.systemImageInfo.Device,
		"channel":      client.systemImageInfo.Channel,
		"build_number": client.systemImageInfo.BuildNumber,
	}
	sess, err := session.NewSession(client.config.Addr,
		client.deriveSessionConfig(info), client.deviceId,
		client.levelMapFactory, client.log)
	if err != nil {
		return err
	}
	client.session = sess
	return nil
}

// levelmapFactory returns a levelMap for the session
func (client *PushClient) levelMapFactory() (levelmap.LevelMap, error) {
	if client.leveldbPath == "" {
		return levelmap.NewLevelMap()
	} else {
		return levelmap.NewSqliteLevelMap(client.leveldbPath)
	}
}

// handleConnState deals with connectivity events
func (client *PushClient) handleConnState(hasConnectivity bool) {
	if client.hasConnectivity == hasConnectivity {
		// nothing to do!
		return
	}
	client.hasConnectivity = hasConnectivity
	if hasConnectivity {
		client.session.AutoRedial(client.sessionConnectedCh)
	} else {
		client.session.Close()
	}
}

// handleErr deals with the session erroring out of its loop
func (client *PushClient) handleErr(err error) {
	// if we're not connected, we don't really care
	client.log.Errorf("session exited: %s", err)
	if client.hasConnectivity {
		client.session.AutoRedial(client.sessionConnectedCh)
	}
}

// filterNotification finds out if the notification is about an actual
// upgrade for the device. It expects msg.Decoded entries to look
// like:
//
// {
// "IMAGE-CHANNEL/DEVICE-MODEL": [BUILD-NUMBER, CHANNEL-ALIAS]
// ...
// }
func (client *PushClient) filterNotification(msg *session.Notification) bool {
	n := len(msg.Decoded)
	if n == 0 {
		return false
	}
	// they are all for us, consider last
	last := msg.Decoded[n-1]
	tag := fmt.Sprintf("%s/%s", client.systemImageInfo.Channel, client.systemImageInfo.Device)
	entry, ok := last[tag]
	if !ok {
		return false
	}
	pair, ok := entry.([]interface{})
	if !ok {
		return false
	}
	if len(pair) < 1 {
		return false
	}
	buildNumber, ok := pair[0].(float64)
	if !ok {
		return false
	}
	curBuildNumber := float64(client.systemImageInfo.BuildNumber)
	if buildNumber > curBuildNumber {
		return true
	}
	// xxx we should really compare channel_target and alias here
	// going backward by a margin, assume switch of target
	if buildNumber < curBuildNumber && (curBuildNumber-buildNumber) > 10 {
		return true
	}
	return false
}

// handleNotification deals with receiving a notification
func (client *PushClient) handleNotification(msg *session.Notification) error {
	if !client.filterNotification(msg) {
		return nil
	}
	action_id := ACTION_ID_SNOWFLAKE
	a := []string{action_id, "Go get it!"} // action value not visible on the phone
	h := map[string]*dbus.Variant{"x-canonical-switch-to-application": &dbus.Variant{true}}
	nots := notifications.Raw(client.notificationsEndp, client.log)
	body := "Tap to open the system updater."
	not_id, err := nots.Notify(
		"ubuntu-push-client",               // app name
		uint32(0),                          // id
		"update_manager_icon",              // icon
		"There's an updated system image.", // summary
		body,           // body
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
func (client *PushClient) handleClick(action_id string) error {
	if action_id != ACTION_ID_SNOWFLAKE {
		return nil
	}
	// it doesn't get much simpler...
	urld := urldispatcher.New(client.urlDispatcherEndp, client.log)
	return urld.DispatchURL("settings:///system/system-update")
}

// doLoop connects events with their handlers
func (client *PushClient) doLoop(connhandler func(bool), clickhandler func(string) error, notifhandler func(*session.Notification) error, errhandler func(error)) {
	for {
		select {
		case state := <-client.connCh:
			connhandler(state)
		case action := <-client.actionsCh:
			clickhandler(action.ActionId)
		case msg := <-client.session.MsgCh:
			notifhandler(msg)
		case err := <-client.session.ErrCh:
			errhandler(err)
		case count := <-client.sessionConnectedCh:
			client.log.Debugf("Session connected after %d attempts", count)
		}
	}
}

// doStart calls each of its arguments in order, returning the first non-nil
// error (or nil at the end)
func (client *PushClient) doStart(fs ...func() error) error {
	for _, f := range fs {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}

// Loop calls doLoop with the "real" handlers
func (client *PushClient) Loop() {
	client.doLoop(client.handleConnState, client.handleClick,
		client.handleNotification, client.handleErr)
}

// Start calls doStart with the "real" starters
func (client *PushClient) Start() error {
	return client.doStart(
		client.configure,
		client.getDeviceId,
		client.takeTheBus,
		client.initSession,
	)
}
