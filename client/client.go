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
	"net/url"
	"os"
	"os/exec"
	"strings"

	"launchpad.net/ubuntu-push/accounts"
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/connectivity"
	"launchpad.net/ubuntu-push/bus/networkmanager"
	"launchpad.net/ubuntu-push/bus/systemimage"
	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/client/service"
	"launchpad.net/ubuntu-push/client/session"
	"launchpad.net/ubuntu-push/client/session/seenstate"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/identifier"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/poller"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/util"
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
	// fallback values for simplified notification usage
	FallbackVibration *launch_helper.Vibration `json:"fallback_vibration"`
	FallbackSound     string                   `json:"fallback_sound"`
	// times for the poller
	PollInterval    config.ConfigTimeDuration `json:"poll_interval"`
	PollSettle      config.ConfigTimeDuration `json:"poll_settle"`
	PollNetworkWait config.ConfigTimeDuration `json:"poll_net_wait"`
	PollPolldWait   config.ConfigTimeDuration `json:"poll_polld_wait"`
	PollDoneWait    config.ConfigTimeDuration `json:"poll_done_wait"`
}

// PushService is the interface we use of service.PushService.
type PushService interface {
	// Start starts the service.
	Start() error
	// Unregister unregisters the token for appId.
	Unregister(appId string) error
}

type PostalService interface {
	// Starts the service
	Start() error
	// Post converts a push message into a presentable notification
	// and a postal message, presents the former and stores the
	// latter in the application's mailbox.
	Post(app *click.AppId, nid string, payload json.RawMessage)
	// IsRunning() returns whether the service is running
	IsRunning() bool
	// Stop() stops the service
	Stop()
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
	connectivityEndp   bus.Endpoint
	systemImageEndp    bus.Endpoint
	systemImageInfo    *systemimage.InfoResult
	connCh             chan bool
	hasConnectivity    bool
	session            *session.ClientSession
	sessionConnectedCh chan uint32
	pushService        PushService
	postalService      PostalService
	unregisterCh       chan *click.AppId
	trackAddressees    map[string]*click.AppId
	installedChecker   click.InstalledChecker
	poller             poller.Poller
	accountsCh         <-chan accounts.Changed
}

// Creates a new Ubuntu Push Notifications client-side daemon that will use
// the given configuration file.
func NewPushClient(configPath string, leveldbPath string) *PushClient {
	client := new(PushClient)
	client.configPath = configPath
	client.leveldbPath = leveldbPath

	return client
}

var newIdentifier = identifier.New

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

	clickUser, err := click.User()
	if err != nil {
		return fmt.Errorf("libclick: %v", err)
	}
	// overridden for testing
	client.installedChecker = clickUser

	client.unregisterCh = make(chan *click.AppId, 10)

	// overridden for testing
	client.idder, err = newIdentifier()
	if err != nil {
		return err
	}
	client.connectivityEndp = bus.SystemBus.Endpoint(networkmanager.BusAddress, client.log)
	client.systemImageEndp = bus.SystemBus.Endpoint(systemimage.BusAddress, client.log)

	client.connCh = make(chan bool, 1)
	client.sessionConnectedCh = make(chan uint32, 1)
	client.accountsCh = accounts.Watch()

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
		PEM:              client.pem,
		Info:             info,
		AuthGetter:       client.getAuthorization,
		AuthURL:          client.config.SessionURL,
		AddresseeChecker: client,
		ErrCh:            make(chan error),
		BroadcastCh:      make(chan *session.BroadcastNotification),
		NotificationsCh:  make(chan session.AddressedNotification),
	}
}

// derivePushServiceSetup derives the service setup from the client configuration bits.
func (client *PushClient) derivePushServiceSetup() (*service.PushServiceSetup, error) {
	setup := new(service.PushServiceSetup)
	purl, err := url.Parse(client.config.RegistrationURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse registration url: %v", err)
	}
	setup.RegURL = purl
	setup.DeviceId = client.deviceId
	setup.AuthGetter = client.getAuthorization
	setup.InstalledChecker = client.installedChecker
	return setup, nil
}

// derivePostalServiceSetup derives the service setup from the client configuration bits.
func (client *PushClient) derivePostalServiceSetup() *service.PostalServiceSetup {
	return &service.PostalServiceSetup{
		InstalledChecker:  client.installedChecker,
		FallbackVibration: client.config.FallbackVibration,
		FallbackSound:     client.config.FallbackSound,
	}
}

// derivePollerSetup derives the Poller setup from the client configuration bits.
func (client *PushClient) derivePollerSetup() *poller.PollerSetup {
	return &poller.PollerSetup{
		Times: poller.Times{
			AlarmInterval:      client.config.PollInterval.TimeDuration(),
			SessionStateSettle: client.config.PollSettle.TimeDuration(),
			NetworkWait:        client.config.PollNetworkWait.TimeDuration(),
			PolldWait:          client.config.PollPolldWait.TimeDuration(),
			DoneWait:           client.config.PollDoneWait.TimeDuration(),
		},
		Log:                client.log,
		SessionStateGetter: client.session,
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

// getDeviceId gets the identifier for the device
func (client *PushClient) getDeviceId() error {
	baseId := client.idder.String()
	b, err := hex.DecodeString(baseId)
	if err != nil {
		return fmt.Errorf("machine-id should be hex: %v", err)
	}
	h := sha256.Sum224(b)
	client.deviceId = base64.StdEncoding.EncodeToString(h[:])
	return nil
}

// takeTheBus starts the connection(s) to D-Bus and sets up associated event channels
func (client *PushClient) takeTheBus() error {
	go connectivity.ConnectedState(client.connectivityEndp,
		client.config.ConnectivityConfig, client.log, client.connCh)
	util.NewAutoRedialer(client.systemImageEndp).Redial()
	sysimg := systemimage.New(client.systemImageEndp, client.log)
	info, err := sysimg.Info()
	if err != nil {
		return err
	}
	client.systemImageInfo = info
	return err
}

// initSessionAndPoller creates the session and the poller objects
func (client *PushClient) initSessionAndPoller() error {
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
	client.poller = poller.New(client.derivePollerSetup())
	return nil
}

// runPoller starts and runs the poller
func (client *PushClient) runPoller() error {
	if err := client.poller.Start(); err != nil {
		return err
	}
	if err := client.poller.Run(); err != nil {
		return err
	}
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

// StartAddresseeBatch starts a batch of checks for addressees.
func (client *PushClient) StartAddresseeBatch() {
	client.trackAddressees = make(map[string]*click.AppId, 10)
}

// CheckForAddressee check for the addressee presence.
func (client *PushClient) CheckForAddressee(notif *protocol.Notification) *click.AppId {
	appId := notif.AppId
	parsed, ok := client.trackAddressees[appId]
	if ok {
		return parsed
	}
	parsed, err := click.ParseAndVerifyAppId(appId, client.installedChecker)
	switch err {
	default:
		client.log.Debugf("notification %#v for invalid app id %#v.", notif.MsgId, notif.AppId)
	case click.ErrMissingApp:
		client.log.Debugf("notification %#v for missing app id %#v.", notif.MsgId, notif.AppId)
		client.unregisterCh <- parsed
		parsed = nil
	case nil:
	}
	client.trackAddressees[appId] = parsed
	return parsed
}

// handleUnregister deals with tokens of uninstalled apps
func (client *PushClient) handleUnregister(app *click.AppId) {
	if !client.installedChecker.Installed(app, false) {
		// xxx small chance of race here, in case the app gets
		// reinstalled and registers itself before we finish
		// the unregister; we need click and app launching
		// collaboration to do better. we redo the hasPackage
		// check here just before to keep the race window as
		// small as possible
		err := client.pushService.Unregister(app.Original()) // XXX WIP
		if err != nil {
			client.log.Errorf("unregistering %s: %s", app, err)
		} else {
			client.log.Debugf("unregistered token for %s", app)
		}
	}
}

// handleConnState deals with connectivity events
func (client *PushClient) handleConnState(hasConnectivity bool) {
	client.log.Debugf("handleConnState: %v", hasConnectivity)
	if client.hasConnectivity == hasConnectivity {
		// nothing to do!
		return
	}
	client.hasConnectivity = hasConnectivity
	client.session.Close()
	if hasConnectivity {
		client.session.AutoRedial(client.sessionConnectedCh)
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

// handleBroadcastNotification deals with receiving a broadcast notification
func (client *PushClient) handleBroadcastNotification(msg *session.BroadcastNotification) error {
	if !client.filterBroadcastNotification(msg) {
		client.log.Debugf("not posting broadcast notification %d; filtered.", msg.TopLevel)
		return nil
	}
	// marshal the last decoded msg to json
	payload, err := json.Marshal(msg.Decoded[len(msg.Decoded)-1])
	if err != nil {
		client.log.Errorf("while posting broadcast notification %d: %v", msg.TopLevel, err)
		return err
	}
	appId, _ := click.ParseAppId("_ubuntu-system-settings")
	client.postalService.Post(appId, "", payload)
	client.log.Debugf("posted broadcast notification %d.", msg.TopLevel)
	return nil
}

// handleUnicastNotification deals with receiving a unicast notification
func (client *PushClient) handleUnicastNotification(anotif session.AddressedNotification) error {
	app := anotif.To
	msg := anotif.Notification
	client.postalService.Post(app, msg.MsgId, msg.Payload)
	client.log.Debugf("posted unicast notification %s for %s.", msg.MsgId, msg.AppId)
	return nil
}

// handleAccountsChange deals with the user adding or removing (or
// changing) the u1 account used to auth
func (client *PushClient) handleAccountsChange() {
	client.log.Infof("U1 account changed; restarting session")
	client.session.ClearCookie()
	client.session.Close()
}

// doLoop connects events with their handlers
func (client *PushClient) doLoop(connhandler func(bool), bcasthandler func(*session.BroadcastNotification) error, ucasthandler func(session.AddressedNotification) error, errhandler func(error), unregisterhandler func(*click.AppId), accountshandler func()) {
	for {
		select {
		case <-client.accountsCh:
			accountshandler()
		case state := <-client.connCh:
			connhandler(state)
		case bcast := <-client.session.BroadcastCh:
			bcasthandler(bcast)
		case aucast := <-client.session.NotificationsCh:
			ucasthandler(aucast)
		case err := <-client.session.ErrCh:
			errhandler(err)
		case count := <-client.sessionConnectedCh:
			client.log.Debugf("session connected after %d attempts", count)
		case app := <-client.unregisterCh:
			unregisterhandler(app)
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
		client.handleBroadcastNotification,
		client.handleUnicastNotification,
		client.handleErr,
		client.handleUnregister,
		client.handleAccountsChange,
	)
}

func (client *PushClient) setupPushService() error {
	setup, err := client.derivePushServiceSetup()
	if err != nil {
		return err
	}

	client.pushService = service.NewPushService(setup, client.log)
	return nil
}

func (client *PushClient) startPushService() error {
	if err := client.pushService.Start(); err != nil {
		return err
	}
	return nil
}

func (client *PushClient) setupPostalService() error {
	setup := client.derivePostalServiceSetup()
	client.postalService = service.NewPostalService(setup, client.log)
	return nil
}

func (client *PushClient) startPostalService() error {
	if err := client.postalService.Start(); err != nil {
		return err
	}
	return nil
}

// Start calls doStart with the "real" starters
func (client *PushClient) Start() error {
	return client.doStart(
		client.configure,
		client.getDeviceId,
		client.setupPushService,
		client.setupPostalService,
		client.startPushService,
		client.startPostalService,
		client.takeTheBus,
		client.initSessionAndPoller,
		client.runPoller,
	)
}
