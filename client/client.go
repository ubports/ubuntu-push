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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"launchpad.net/go-dbus/v1"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/connectivity"
	"launchpad.net/ubuntu-push/bus/networkmanager"
	"launchpad.net/ubuntu-push/bus/notifications"
	"launchpad.net/ubuntu-push/bus/systemimage"
	"launchpad.net/ubuntu-push/bus/urldispatcher"
	"launchpad.net/ubuntu-push/client/service"
	"launchpad.net/ubuntu-push/client/session"
	"launchpad.net/ubuntu-push/client/session/seenstate"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/util"
	"launchpad.net/ubuntu-push/whoopsie/identifier"
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
	// How to invoke the auth helper
	AuthHelper      string `json:"auth_helper"`
	SessionURL      string `json:"session_url"`
	RegistrationURL string `json:"registration_url"`
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
	serviceEndpoint    bus.Endpoint
	service            *service.PushService
	postalEndpoint     bus.Endpoint
	postal             *service.PostalService
}

var (
	system_update_url   = "settings:///system/system-update"
	ACTION_ID_SNOWFLAKE = "::ubuntu-push-client::"
	ACTION_ID_BROADCAST = ACTION_ID_SNOWFLAKE + system_update_url
)

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
		PEM:        client.pem,
		Info:       info,
		AuthGetter: client.getAuthorization,
		AuthURL:    client.config.SessionURL,
	}
}

// getAuthorization gets the authorization blob to send to the server
func (client *PushClient) getAuthorization(url string) string {
	client.log.Debugf("getting authorization for %s", url)
	// using a helper, for now at least
	if len(client.config.AuthHelper) == 0 {
		// do nothing if helper is unset or empty
		return ""
	}

	auth, err := exec.Command(client.config.AuthHelper, url).Output()
	if err != nil {
		// For now we just log the error, as we don't want to block unauthorized users
		client.log.Errorf("unable to get the authorization token from the account: %v", err)
		return ""
	} else {
		return strings.TrimSpace(string(auth))
	}
}

// getDeviceId gets the whoopsie identifier for the device
func (client *PushClient) getDeviceId() error {
	err := client.idder.Generate()
	if err != nil {
		return err
	}
	baseId := client.idder.String()
	b, err := hex.DecodeString(baseId)
	if err != nil {
		return fmt.Errorf("whoopsie id should be hex: %v", err)
	}
	h := sha256.Sum224(b)
	client.deviceId = base64.StdEncoding.EncodeToString(h[:])
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
		client.seenStateFactory, client.log)
	if err != nil {
		return err
	}
	client.session = sess
	return nil
}

// seenStateFactory returns a SeenState for the session
func (client *PushClient) seenStateFactory() (seenstate.SeenState, error) {
	if client.leveldbPath == "" {
		return seenstate.NewSeenState()
	} else {
		return seenstate.NewSqliteSeenState(client.leveldbPath)
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

// filterBroadcastNotification finds out if the notification is about an actual
// upgrade for the device. It expects msg.Decoded entries to look
// like:
//
// {
// "IMAGE-CHANNEL/DEVICE-MODEL": [BUILD-NUMBER, CHANNEL-ALIAS]
// ...
// }
func (client *PushClient) filterBroadcastNotification(msg *session.BroadcastNotification) bool {
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

func (client *PushClient) sendNotification(action_id, icon, summary, body string) (uint32, error) {
	a := []string{action_id, "Switch to app"} // action value not visible on the phone
	h := map[string]*dbus.Variant{"x-canonical-switch-to-application": &dbus.Variant{true}}
	nots := notifications.Raw(client.notificationsEndp, client.log)
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

// handleBroadcastNotification deals with receiving a broadcast notification
func (client *PushClient) handleBroadcastNotification(msg *session.BroadcastNotification) error {
	if !client.filterBroadcastNotification(msg) {
		return nil
	}
	not_id, err := client.sendNotification(ACTION_ID_BROADCAST,
		"update_manager_icon", "There's an updated system image.",
		"Tap to open the system updater.")
	if err != nil {
		client.log.Errorf("showing notification: %s", err)
		return err
	}
	client.log.Debugf("got notification id %d", not_id)
	return nil
}

// handleUnicastNotification deals with receiving a unicast notification
func (client *PushClient) handleUnicastNotification(msg *protocol.Notification) error {
	client.log.Debugf("sending notification %#v for %#v.", msg.MsgId, msg.AppId)
	return client.postal.Inject(msg.AppId, string(msg.Payload))
}

// handleClick deals with the user clicking a notification
func (client *PushClient) handleClick(action_id string) error {
	// “The string is a stark data structure and everywhere it is passed
	// there is much duplication of process. It is a perfect vehicle for
	// hiding information.”
	//
	// From ACM's SIGPLAN publication, (September, 1982), Article
	// "Epigrams in Programming", by Alan J. Perlis of Yale University.
	url := strings.TrimPrefix(action_id, ACTION_ID_SNOWFLAKE)
	if len(url) == len(action_id) || len(url) == 0 {
		// it didn't start with the prefix
		return nil
	}
	// it doesn't get much simpler...
	urld := urldispatcher.New(client.urlDispatcherEndp, client.log)
	return urld.DispatchURL(url)
}

// doLoop connects events with their handlers
func (client *PushClient) doLoop(connhandler func(bool), clickhandler func(string) error, bcasthandler func(*session.BroadcastNotification) error, ucasthandler func(*protocol.Notification) error, errhandler func(error)) {
	for {
		select {
		case state := <-client.connCh:
			connhandler(state)
		case action := <-client.actionsCh:
			clickhandler(action.ActionId)
		case bcast := <-client.session.BroadcastCh:
			bcasthandler(bcast)
		case ucast := <-client.session.NotificationsCh:
			ucasthandler(ucast)
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
	client.doLoop(client.handleConnState,
		client.handleClick,
		client.handleBroadcastNotification,
		client.handleUnicastNotification,
		client.handleErr)
}

// these are the currently supported fields of a unicast message
type UnicastMessage struct {
	Icon    string          `json:"icon"`
	Body    string          `json:"body"`
	Summary string          `json:"summary"`
	URL     string          `json:"url"`
	Blob    json.RawMessage `json:"blob"`
}

func (client *PushClient) messageHandler(message []byte) error {
	var umsg = new(UnicastMessage)
	err := json.Unmarshal(message, &umsg)
	if err != nil {
		client.log.Errorf("unable to unmarshal message: %v", err)
		return err
	}

	not_id, err := client.sendNotification(
		ACTION_ID_SNOWFLAKE+umsg.URL,
		umsg.Icon, umsg.Summary, umsg.Body)

	if err != nil {
		client.log.Errorf("showing notification: %s", err)
		return err
	}
	client.log.Debugf("got notification id %d", not_id)
	return nil
}

func (client *PushClient) startService() error {
	if client.serviceEndpoint == nil {
		client.serviceEndpoint = bus.SessionBus.Endpoint(service.PushServiceBusAddress, client.log)
	}
	if client.postalEndpoint == nil {
		client.postalEndpoint = bus.SessionBus.Endpoint(service.PostalServiceBusAddress, client.log)
	}

	client.service = service.NewPushService(client.serviceEndpoint, client.log)
	client.service.SetRegistrationURL(client.config.RegistrationURL)
	client.service.SetAuthGetter(client.getAuthorization)
	client.postal = service.NewPostalService(client.postalEndpoint, client.log)
	client.postal.SetMessageHandler(client.messageHandler)
	if err := client.service.Start(); err != nil {
		return err
	}
	if err := client.postal.Start(); err != nil {
		return err
	}
	return nil
}

// Start calls doStart with the "real" starters
func (client *PushClient) Start() error {
	return client.doStart(
		client.configure,
		client.startService,
		client.getDeviceId,
		client.takeTheBus,
		client.initSession,
	)
}
