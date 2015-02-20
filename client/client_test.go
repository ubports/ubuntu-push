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

package client

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	//"runtime"
	"testing"
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/accounts"
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/networkmanager"
	"launchpad.net/ubuntu-push/bus/systemimage"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/click"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	"launchpad.net/ubuntu-push/client/service"
	"launchpad.net/ubuntu-push/client/session"
	"launchpad.net/ubuntu-push/client/session/seenstate"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/identifier"
	idtesting "launchpad.net/ubuntu-push/identifier/testing"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/poller"
	"launchpad.net/ubuntu-push/protocol"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
	"launchpad.net/ubuntu-push/util"
)

func TestClient(t *testing.T) { TestingT(t) }

// takeNext takes a value from given channel with a 5s timeout
func takeNextBool(ch <-chan bool) bool {
	select {
	case <-time.After(5 * time.Second):
		panic("channel stuck: too long waiting")
	case v := <-ch:
		return v
	}
}

type dumbCommon struct {
	startCount   int
	stopCount    int
	runningCount int
	running      bool
	err          error
}

func (d *dumbCommon) Start() error {
	d.startCount++
	return d.err
}
func (d *dumbCommon) Stop() {
	d.stopCount++
}
func (d *dumbCommon) IsRunning() bool {
	d.runningCount++
	return d.running
}

type dumbPush struct {
	dumbCommon
	unregCount int
	unregArgs  []string
}

func (d *dumbPush) Unregister(appId string) error {
	d.unregCount++
	d.unregArgs = append(d.unregArgs, appId)
	return d.err
}

type postArgs struct {
	app     *click.AppId
	nid     string
	payload json.RawMessage
}

type dumbPostal struct {
	dumbCommon
	bcastCount int
	postCount  int
	postArgs   []postArgs
}

func (d *dumbPostal) Post(app *click.AppId, nid string, payload json.RawMessage) {
	d.postCount++
	if app.Application == "ubuntu-system-settings" {
		d.bcastCount++
	}
	d.postArgs = append(d.postArgs, postArgs{app, nid, payload})
}

var _ PostalService = (*dumbPostal)(nil)
var _ PushService = (*dumbPush)(nil)

type clientSuite struct {
	timeouts    []time.Duration
	configPath  string
	leveldbPath string
	log         *helpers.TestLogger
}

var _ = Suite(&clientSuite{})

const (
	staticText = "something ipsum dolor something"
	staticHash = "6155f83b471583f47c99998a472a178f"
)

func mkHandler(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		w.Write([]byte(text))
		w.(http.Flusher).Flush()
	}
}

func (cs *clientSuite) SetUpSuite(c *C) {
	config.IgnoreParsedFlags = true // because configure() uses <flags>
	newIdentifier = func() (identifier.Id, error) {
		id := idtesting.Settable()
		id.Set("42") // must be hex of len 32
		return id, nil
	}
	cs.timeouts = util.SwapTimeouts([]time.Duration{0})
	cs.leveldbPath = ""
}

func (cs *clientSuite) TearDownSuite(c *C) {
	util.SwapTimeouts(cs.timeouts)
	cs.timeouts = nil
	newIdentifier = identifier.New
}

func (cs *clientSuite) writeTestConfig(overrides map[string]interface{}) {
	pem_file := helpers.SourceRelative("../server/acceptance/ssl/testing.cert")
	cfgMap := map[string]interface{}{
		"fallback_vibration":     &launch_helper.Vibration{Pattern: []uint32{1}},
		"fallback_sound":         "sounds/ubuntu/notifications/Blip.ogg",
		"connect_timeout":        "7ms",
		"exchange_timeout":       "10ms",
		"hosts_cache_expiry":     "1h",
		"expect_all_repaired":    "30m",
		"stabilizing_timeout":    "0ms",
		"connectivity_check_url": "",
		"connectivity_check_md5": "",
		"addr":             ":0",
		"cert_pem_file":    pem_file,
		"recheck_timeout":  "3h",
		"auth_helper":      "",
		"session_url":      "xyzzy://",
		"registration_url": "reg://",
		"log_level":        "debug",
		"poll_interval":    "5m",
		"poll_settle":      "20ms",
		"poll_net_wait":    "1m",
		"poll_polld_wait":  "3m",
		"poll_done_wait":   "5s",
	}
	for k, v := range overrides {
		cfgMap[k] = v
	}
	cfgBlob, err := json.Marshal(cfgMap)
	if err != nil {
		panic(err)
	}
	ioutil.WriteFile(cs.configPath, cfgBlob, 0600)
}

func (cs *clientSuite) SetUpTest(c *C) {
	cs.log = helpers.NewTestLogger(c, "debug")
	dir := c.MkDir()
	cs.configPath = filepath.Join(dir, "config")

	cs.writeTestConfig(nil)
}

func (cs *clientSuite) TearDownTest(c *C) {
	//fmt.Println("GOROUTINE# ", runtime.NumGoroutine())
	/*
	   var x [16*1024]byte
	   sz := runtime.Stack(x[:], true)
	   fmt.Println(string(x[:sz]))
	*/
}

type sqlientSuite struct{ clientSuite }

func (s *sqlientSuite) SetUpSuite(c *C) {
	s.clientSuite.SetUpSuite(c)
	s.leveldbPath = ":memory:"
}

var _ = Suite(&sqlientSuite{})

/*****************************************************************
    configure tests
******************************************************************/

func (cs *clientSuite) TestConfigureWorks(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, IsNil)
	c.Assert(cli.config, NotNil)
	c.Check(cli.config.ExchangeTimeout.TimeDuration(), Equals, time.Duration(10*time.Millisecond))
}

func (cs *clientSuite) TestConfigureWorksWithFlags(c *C) {
	flag.CommandLine = flag.NewFlagSet("client", flag.ContinueOnError)
	os.Args = []string{"client", "-addr", "foo:7777"}
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, IsNil)
	c.Assert(cli.config, NotNil)
	c.Check(cli.config.Addr, Equals, "foo:7777")
}

func (cs *clientSuite) TestConfigureSetsUpLog(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	c.Check(cli.log, IsNil)
	err := cli.configure()
	c.Assert(err, IsNil)
	c.Assert(cli.log, NotNil)
}

func (cs *clientSuite) TestConfigureSetsUpPEM(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	c.Check(cli.pem, IsNil)
	err := cli.configure()
	c.Assert(err, IsNil)
	c.Assert(cli.pem, NotNil)
}

func (cs *clientSuite) TestConfigureSetsUpIdder(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	c.Check(cli.idder, IsNil)
	err := cli.configure()
	c.Assert(err, IsNil)
	c.Assert(cli.idder, NotNil)
}

func (cs *clientSuite) TestConfigureSetsUpEndpoints(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)

	c.Check(cli.connectivityEndp, IsNil)
	c.Check(cli.systemImageEndp, IsNil)
	err := cli.configure()
	c.Assert(err, IsNil)
	c.Check(cli.connectivityEndp, NotNil)
	c.Check(cli.systemImageEndp, NotNil)
}

func (cs *clientSuite) TestConfigureSetsUpConnCh(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	c.Check(cli.connCh, IsNil)
	err := cli.configure()
	c.Assert(err, IsNil)
	c.Assert(cli.connCh, NotNil)
}

func (cs *clientSuite) TestConfigureSetsUpAddresseeChecks(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	c.Check(cli.unregisterCh, IsNil)
	err := cli.configure()
	c.Assert(err, IsNil)
	c.Assert(cli.unregisterCh, NotNil)
	app, err := click.ParseAppId("com.bar.baz_foo")
	c.Assert(err, IsNil)
	c.Assert(cli.installedChecker.Installed(app, false), Equals, false)
}

func (cs *clientSuite) TestConfigureBailsOnBadFilename(c *C) {
	cli := NewPushClient("/does/not/exist", cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, NotNil)
}

func (cs *clientSuite) TestConfigureBailsOnBadConfig(c *C) {
	cli := NewPushClient("/etc/passwd", cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, NotNil)
}

func (cs *clientSuite) TestConfigureBailsOnBadPEMFilename(c *C) {
	cs.writeTestConfig(map[string]interface{}{
		"cert_pem_file": "/a/b/c",
	})
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, ErrorMatches, "reading PEM file: .*")
}

func (cs *clientSuite) TestConfigureBailsOnBadPEM(c *C) {
	cs.writeTestConfig(map[string]interface{}{
		"cert_pem_file": "/etc/passwd",
	})
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, ErrorMatches, "no PEM found.*")
}

func (cs *clientSuite) TestConfigureBailsOnNoHosts(c *C) {
	cs.writeTestConfig(map[string]interface{}{
		"addr": "  ",
	})
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, ErrorMatches, "no hosts specified")
}

func (cs *clientSuite) TestConfigureRemovesBlanksInAddr(c *C) {
	cs.writeTestConfig(map[string]interface{}{
		"addr": " foo: 443",
	})
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, IsNil)
	c.Check(cli.config.Addr, Equals, "foo:443")
}

/*****************************************************************
    addresses checking tests
******************************************************************/

type testInstalledChecker func(*click.AppId, bool) bool

func (tic testInstalledChecker) Installed(app *click.AppId, setVersion bool) bool {
	return tic(app, setVersion)
}

var (
	appId1     = "com.example.app1_app1"
	appId2     = "com.example.app2_app2"
	appIdHello = "com.example.test_hello"
	app1       = clickhelp.MustParseAppId(appId1)
	app2       = clickhelp.MustParseAppId(appId2)
	appHello   = clickhelp.MustParseAppId(appIdHello)
)

func (cs *clientSuite) TestCheckForAddressee(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.unregisterCh = make(chan *click.AppId, 5)
	cli.StartAddresseeBatch()
	calls := 0
	cli.installedChecker = testInstalledChecker(func(app *click.AppId, setVersion bool) bool {
		calls++
		c.Assert(setVersion, Equals, true)
		if app.Original() == appId1 {
			return false
		}
		return true
	})

	c.Check(cli.CheckForAddressee(&protocol.Notification{AppId: "bad-id"}), IsNil)
	c.Check(calls, Equals, 0)
	c.Assert(cli.unregisterCh, HasLen, 0)
	c.Check(cs.log.Captured(), Matches, `DEBUG notification "" for invalid app id "bad-id".\n`)
	cs.log.ResetCapture()

	c.Check(cli.CheckForAddressee(&protocol.Notification{AppId: appId1}), IsNil)
	c.Check(calls, Equals, 1)
	c.Assert(cli.unregisterCh, HasLen, 1)
	c.Check(<-cli.unregisterCh, DeepEquals, app1)
	c.Check(cs.log.Captured(), Matches, `DEBUG notification "" for missing app id "com.example.app1_app1".\n`)
	cs.log.ResetCapture()

	c.Check(cli.CheckForAddressee(&protocol.Notification{AppId: appId2}), DeepEquals, app2)
	c.Check(calls, Equals, 2)
	c.Check(cs.log.Captured(), Matches, "")
	cs.log.ResetCapture()

	c.Check(cli.CheckForAddressee(&protocol.Notification{AppId: appId1}), IsNil)
	c.Check(calls, Equals, 2)
	c.Check(cli.CheckForAddressee(&protocol.Notification{AppId: appId2}), DeepEquals, app2)
	c.Check(calls, Equals, 2)
	c.Check(cli.unregisterCh, HasLen, 0)
	c.Check(cs.log.Captured(), Matches, "")
}

/*****************************************************************
    deriveSessionConfig tests
******************************************************************/

func (cs *clientSuite) TestDeriveSessionConfig(c *C) {
	cs.writeTestConfig(map[string]interface{}{
		"auth_helper": "auth helper",
	})
	info := map[string]interface{}{
		"foo": 1,
	}
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, IsNil)
	expected := session.ClientSessionConfig{
		ConnectTimeout:         7 * time.Millisecond,
		ExchangeTimeout:        10 * time.Millisecond,
		HostsCachingExpiryTime: 1 * time.Hour,
		ExpectAllRepairedTime:  30 * time.Minute,
		PEM:              cli.pem,
		Info:             info,
		AuthGetter:       func(string) string { return "" },
		AuthURL:          "xyzzy://",
		AddresseeChecker: cli,
	}
	// sanity check that we are looking at all fields
	vExpected := reflect.ValueOf(expected)
	nf := vExpected.NumField()
	for i := 0; i < nf; i++ {
		fv := vExpected.Field(i)
		// field isn't empty/zero
		c.Assert(fv.Interface(), Not(DeepEquals), reflect.Zero(fv.Type()).Interface(), Commentf("forgot about: %s", vExpected.Type().Field(i).Name))
	}
	// finally compare
	conf := cli.deriveSessionConfig(info)
	// compare authGetter by string
	c.Check(fmt.Sprintf("%#v", conf.AuthGetter), Equals, fmt.Sprintf("%#v", cli.getAuthorization))
	// and set it to nil
	conf.AuthGetter = nil
	expected.AuthGetter = nil
	c.Check(conf, DeepEquals, expected)
}

/*****************************************************************
    derivePushServiceSetup tests
******************************************************************/

func (cs *clientSuite) TestDerivePushServiceSetup(c *C) {
	cs.writeTestConfig(map[string]interface{}{})
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, IsNil)
	cli.deviceId = "zoo"
	expected := &service.PushServiceSetup{
		DeviceId:         "zoo",
		AuthGetter:       func(string) string { return "" },
		RegURL:           helpers.ParseURL("reg://"),
		InstalledChecker: cli.installedChecker,
	}
	// sanity check that we are looking at all fields
	vExpected := reflect.ValueOf(expected).Elem()
	nf := vExpected.NumField()
	for i := 0; i < nf; i++ {
		fv := vExpected.Field(i)
		// field isn't empty/zero
		c.Assert(fv.Interface(), Not(DeepEquals), reflect.Zero(fv.Type()).Interface(), Commentf("forgot about: %s", vExpected.Type().Field(i).Name))
	}
	// finally compare
	setup, err := cli.derivePushServiceSetup()
	c.Assert(err, IsNil)
	// compare authGetter by string
	c.Check(fmt.Sprintf("%#v", setup.AuthGetter), Equals, fmt.Sprintf("%#v", cli.getAuthorization))
	// and set it to nil
	setup.AuthGetter = nil
	expected.AuthGetter = nil
	c.Check(setup, DeepEquals, expected)
}

func (cs *clientSuite) TestDerivePushServiceSetupError(c *C) {
	cs.writeTestConfig(map[string]interface{}{
		"registration_url": "%gh",
	})
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, IsNil)
	_, err = cli.derivePushServiceSetup()
	c.Check(err, ErrorMatches, "cannot parse registration url:.*")
}

/*****************************************************************
    derivePostalConfig tests
******************************************************************/
func (cs *clientSuite) TestDerivePostalServiceSetup(c *C) {
	cs.writeTestConfig(map[string]interface{}{})
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, IsNil)
	expected := &service.PostalServiceSetup{
		InstalledChecker:  cli.installedChecker,
		FallbackVibration: cli.config.FallbackVibration,
		FallbackSound:     cli.config.FallbackSound,
	}
	// sanity check that we are looking at all fields
	vExpected := reflect.ValueOf(expected).Elem()
	nf := vExpected.NumField()
	for i := 0; i < nf; i++ {
		fv := vExpected.Field(i)
		// field isn't empty/zero
		c.Assert(fv.Interface(), Not(DeepEquals), reflect.Zero(fv.Type()).Interface(), Commentf("forgot about: %s", vExpected.Type().Field(i).Name))
	}
	// finally compare
	setup := cli.derivePostalServiceSetup()
	c.Check(setup, DeepEquals, expected)
}

/*****************************************************************
    derivePollerSetup tests
******************************************************************/
func (cs *clientSuite) TestDerivePollerSetup(c *C) {
	cs.writeTestConfig(map[string]interface{}{})
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.session = new(session.ClientSession)
	err := cli.configure()
	c.Assert(err, IsNil)
	expected := &poller.PollerSetup{
		Times: poller.Times{
			AlarmInterval:      5 * time.Minute,
			SessionStateSettle: 20 * time.Millisecond,
			NetworkWait:        time.Minute,
			PolldWait:          3 * time.Minute,
			DoneWait:           5 * time.Second,
		},
		Log:                cli.log,
		SessionStateGetter: cli.session,
	}
	// sanity check that we are looking at all fields
	vExpected := reflect.ValueOf(expected).Elem()
	nf := vExpected.NumField()
	for i := 0; i < nf; i++ {
		fv := vExpected.Field(i)
		// field isn't empty/zero
		c.Assert(fv.Interface(), Not(DeepEquals), reflect.Zero(fv.Type()).Interface(), Commentf("forgot about: %s", vExpected.Type().Field(i).Name))
	}
	// finally compare
	setup := cli.derivePollerSetup()
	c.Check(setup, DeepEquals, expected)
}

/*****************************************************************
    startService tests
******************************************************************/

func (cs *clientSuite) TestStartPushServiceCallsStart(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	d := new(dumbPush)
	cli.pushService = d
	c.Check(cli.startPushService(), IsNil)
	c.Check(d.startCount, Equals, 1)
}

func (cs *clientSuite) TestStartPostServiceCallsStart(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	d := new(dumbPostal)
	cli.postalService = d
	c.Check(cli.startPostalService(), IsNil)
	c.Check(d.startCount, Equals, 1)
}

func (cs *clientSuite) TestSetupPushServiceSetupError(c *C) {
	cs.writeTestConfig(map[string]interface{}{
		"registration_url": "%gh",
	})
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	c.Assert(err, IsNil)
	err = cli.setupPushService()
	c.Check(err, ErrorMatches, "cannot parse registration url:.*")
}

func (cs *clientSuite) TestSetupPushService(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	c.Assert(cli.configure(), IsNil)
	c.Check(cli.pushService, IsNil)
	c.Check(cli.setupPushService(), IsNil)
	c.Check(cli.pushService, NotNil)
}

func (cs *clientSuite) TestStartPushErrorsOnPushStartError(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	d := new(dumbPush)
	err := errors.New("potato")
	d.err = err
	cli.pushService = d
	c.Check(cli.startPushService(), Equals, err)
	c.Check(d.startCount, Equals, 1)
}

func (cs *clientSuite) TestStartPostalErrorsOnPostalStartError(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	d := new(dumbPostal)
	err := errors.New("potato")
	d.err = err
	cli.postalService = d
	c.Check(cli.startPostalService(), Equals, err)
	c.Check(d.startCount, Equals, 1)
}

/*****************************************************************
    getDeviceId tests
******************************************************************/

func (cs *clientSuite) TestGetDeviceIdWorks(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.idder, _ = identifier.New()
	c.Check(cli.deviceId, Equals, "")
	c.Check(cli.getDeviceId(), IsNil)
	c.Check(cli.deviceId, HasLen, 40)
}

func (cs *clientSuite) TestGetDeviceIdCanFail(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.idder = idtesting.Failing()
	c.Check(cli.deviceId, Equals, "")
	c.Check(cli.getDeviceId(), NotNil)
}

func (cs *clientSuite) TestGetDeviceIdIdentifierDoesTheUnexpected(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	settable := idtesting.Settable()
	cli.idder = settable
	settable.Set("not-hex")
	c.Check(cli.deviceId, Equals, "")
	c.Check(cli.getDeviceId(), ErrorMatches, "machine-id should be hex: .*")
}

/*****************************************************************
    takeTheBus tests
******************************************************************/

func (cs *clientSuite) TestTakeTheBusWorks(c *C) {
	// http server used for connectivity test
	ts := httptest.NewServer(mkHandler(staticText))
	defer ts.Close()

	// testing endpoints
	cCond := condition.Fail2Work(7)
	cEndp := testibus.NewTestingEndpoint(cCond, condition.Work(true),
		uint32(networkmanager.Connecting),
	)
	siCond := condition.Fail2Work(2)
	siEndp := testibus.NewMultiValuedTestingEndpoint(siCond, condition.Work(true), []interface{}{int32(101), "mako", "daily", "Unknown", map[string]string{}})
	tickerCh := make(chan []interface{})
	nopTickerCh := make(chan []interface{})
	testibus.SetWatchSource(cEndp, "StateChanged", tickerCh)
	testibus.SetWatchSource(cEndp, "PropertiesChanged", nopTickerCh)
	defer close(tickerCh)
	defer close(nopTickerCh)
	// ok, create the thing
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	err := cli.configure()
	c.Assert(err, IsNil)

	// and stomp on things for testing
	cli.config.ConnectivityConfig.ConnectivityCheckURL = ts.URL
	cli.config.ConnectivityConfig.ConnectivityCheckMD5 = staticHash
	cli.connectivityEndp = cEndp
	cli.systemImageEndp = siEndp

	c.Assert(cli.takeTheBus(), IsNil)

	c.Check(takeNextBool(cli.connCh), Equals, false)
	tickerCh <- []interface{}{uint32(networkmanager.ConnectedGlobal)}
	c.Check(takeNextBool(cli.connCh), Equals, true)
	// the connectivity endpoint retried until connected
	c.Check(cCond.OK(), Equals, true)
	// the systemimage endpoint retried until connected
	c.Check(siCond.OK(), Equals, true)
}

// takeTheBus can, in fact, fail
func (cs *clientSuite) TestTakeTheBusCanFail(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	err := cli.configure()
	cli.log = cs.log
	c.Assert(err, IsNil)

	// and stomp on things for testing
	cli.connectivityEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))
	cli.systemImageEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))

	c.Check(cli.takeTheBus(), NotNil)
	c.Assert(cli.setupPostalService(), IsNil)
}

/*****************************************************************
    handleErr tests
******************************************************************/

func (cs *clientSuite) TestHandleErr(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSessionAndPoller(), IsNil)
	cs.log.ResetCapture()
	cli.hasConnectivity = true
	defer cli.session.Close()
	cli.handleErr(errors.New("bananas"))
	c.Check(cs.log.Captured(), Matches, ".*session exited.*bananas\n")
}

/*****************************************************************
    seenStateFactory tests
******************************************************************/

func (cs *clientSuite) TestSeenStateFactoryNoDbPath(c *C) {
	cli := NewPushClient(cs.configPath, "")
	ln, err := cli.seenStateFactory()
	c.Assert(err, IsNil)
	defer ln.Close()
	c.Check(fmt.Sprintf("%T", ln), Equals, "*seenstate.memSeenState")
}

func (cs *clientSuite) TestSeenStateFactoryWithDbPath(c *C) {
	cli := NewPushClient(cs.configPath, ":memory:")
	ln, err := cli.seenStateFactory()
	c.Assert(err, IsNil)
	defer ln.Close()
	c.Check(fmt.Sprintf("%T", ln), Equals, "*seenstate.sqliteSeenState")
}

/*****************************************************************
    handleConnState tests
******************************************************************/

func (cs *clientSuite) TestHandleConnStateD2C(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSessionAndPoller(), IsNil)

	c.Assert(cli.hasConnectivity, Equals, false)
	defer cli.session.Close()
	cli.handleConnState(true)
	c.Check(cli.hasConnectivity, Equals, true)
	c.Assert(cli.session, NotNil)
}

func (cs *clientSuite) TestHandleConnStateSame(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	// here we want to check that we don't do anything
	c.Assert(cli.session, IsNil)
	c.Assert(cli.hasConnectivity, Equals, false)
	cli.handleConnState(false)
	c.Check(cli.session, IsNil)

	cli.hasConnectivity = true
	cli.handleConnState(true)
	c.Check(cli.session, IsNil)
}

func (cs *clientSuite) TestHandleConnStateC2D(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.session, _ = session.NewSession(cli.config.Addr, cli.deriveSessionConfig(nil), cli.deviceId, seenstate.NewSeenState, cs.log)
	cli.session.Dial()
	cli.hasConnectivity = true

	// cli.session.State() will be "Error" here, for now at least
	c.Check(cli.session.State(), Not(Equals), session.Disconnected)
	cli.handleConnState(false)
	c.Check(cli.session.State(), Equals, session.Disconnected)
}

func (cs *clientSuite) TestHandleConnStateC2DPending(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.session, _ = session.NewSession(cli.config.Addr, cli.deriveSessionConfig(nil), cli.deviceId, seenstate.NewSeenState, cs.log)
	cli.hasConnectivity = true

	cli.handleConnState(false)
	c.Check(cli.session.State(), Equals, session.Disconnected)
}

/*****************************************************************
   filterBroadcastNotification tests
******************************************************************/

var siInfoRes = &systemimage.InfoResult{
	Device:      "mako",
	Channel:     "daily",
	BuildNumber: 102,
	LastUpdate:  "Unknown",
}

func (cs *clientSuite) TestFilterBroadcastNotification(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.systemImageInfo = siInfoRes
	// empty
	msg := &session.BroadcastNotification{}
	c.Check(cli.filterBroadcastNotification(msg), Equals, false)
	// same build number
	msg = &session.BroadcastNotification{
		Decoded: []map[string]interface{}{
			map[string]interface{}{
				"daily/mako": []interface{}{float64(102), "tubular"},
			},
		},
	}
	c.Check(cli.filterBroadcastNotification(msg), Equals, false)
	// higher build number and pick last
	msg = &session.BroadcastNotification{
		Decoded: []map[string]interface{}{
			map[string]interface{}{
				"daily/mako": []interface{}{float64(102), "tubular"},
			},
			map[string]interface{}{
				"daily/mako": []interface{}{float64(103), "tubular"},
			},
		},
	}
	c.Check(cli.filterBroadcastNotification(msg), Equals, true)
	// going backward by a margin, assume switch of alias
	msg = &session.BroadcastNotification{
		Decoded: []map[string]interface{}{
			map[string]interface{}{
				"daily/mako": []interface{}{float64(102), "tubular"},
			},
			map[string]interface{}{
				"daily/mako": []interface{}{float64(2), "urban"},
			},
		},
	}
	c.Check(cli.filterBroadcastNotification(msg), Equals, true)
}

func (cs *clientSuite) TestFilterBroadcastNotificationRobust(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.systemImageInfo = siInfoRes
	msg := &session.BroadcastNotification{
		Decoded: []map[string]interface{}{
			map[string]interface{}{},
		},
	}
	c.Check(cli.filterBroadcastNotification(msg), Equals, false)
	for _, broken := range []interface{}{
		5,
		[]interface{}{},
		[]interface{}{55},
	} {
		msg := &session.BroadcastNotification{
			Decoded: []map[string]interface{}{
				map[string]interface{}{
					"daily/mako": broken,
				},
			},
		}
		c.Check(cli.filterBroadcastNotification(msg), Equals, false)
	}
}

/*****************************************************************
    handleBroadcastNotification tests
******************************************************************/

var (
	positiveBroadcastNotification = &session.BroadcastNotification{
		Decoded: []map[string]interface{}{
			map[string]interface{}{
				"daily/mako": []interface{}{float64(102), "tubular"},
			},
			map[string]interface{}{
				"daily/mako": []interface{}{float64(103), "tubular"},
			},
		},
	}
	negativeBroadcastNotification = &session.BroadcastNotification{
		Decoded: []map[string]interface{}{
			map[string]interface{}{
				"daily/mako": []interface{}{float64(102), "tubular"},
			},
		},
	}
)

func (cs *clientSuite) TestHandleBroadcastNotification(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.systemImageInfo = siInfoRes
	cli.log = cs.log
	d := new(dumbPostal)
	cli.postalService = d
	c.Check(cli.handleBroadcastNotification(positiveBroadcastNotification), IsNil)
	// we dun posted
	c.Check(d.bcastCount, Equals, 1)
	c.Assert(d.postArgs, HasLen, 1)
	expectedApp, _ := click.ParseAppId("_ubuntu-system-settings")
	c.Check(d.postArgs[0].app, DeepEquals, expectedApp)
	c.Check(d.postArgs[0].nid, Equals, "")
	expectedData, _ := json.Marshal(positiveBroadcastNotification.Decoded[1])
	c.Check([]byte(d.postArgs[0].payload), DeepEquals, expectedData)
}

func (cs *clientSuite) TestHandleBroadcastNotificationNothingToDo(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.systemImageInfo = siInfoRes
	cli.log = cs.log
	d := new(dumbPostal)
	cli.postalService = d
	c.Check(cli.handleBroadcastNotification(negativeBroadcastNotification), IsNil)
	// we not dun no posted
	c.Check(d.bcastCount, Equals, 0)
}

/*****************************************************************
    handleUnicastNotification tests
******************************************************************/

var payload = `{"message": "aGVsbG8=", "notification": {"card": {"icon": "icon-value", "summary": "summary-value", "body": "body-value", "actions": []}}}`
var notif = &protocol.Notification{AppId: appIdHello, Payload: []byte(payload), MsgId: "42"}

func (cs *clientSuite) TestHandleUcastNotification(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	d := new(dumbPostal)
	cli.postalService = d

	c.Check(cli.handleUnicastNotification(session.AddressedNotification{appHello, notif}), IsNil)
	// check we sent the notification
	c.Check(d.postCount, Equals, 1)
	c.Assert(d.postArgs, HasLen, 1)
	c.Check(d.postArgs[0].app, Equals, appHello)
	c.Check(d.postArgs[0].nid, Equals, notif.MsgId)
	c.Check(d.postArgs[0].payload, DeepEquals, notif.Payload)
}

/*****************************************************************
    handleUnregister tests
******************************************************************/

type testPushService struct {
	err          error
	unregistered string
}

func (ps *testPushService) Start() error {
	return nil
}

func (ps *testPushService) Unregister(appId string) error {
	ps.unregistered = appId
	return ps.err
}

func (cs *clientSuite) TestHandleUnregister(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.installedChecker = testInstalledChecker(func(app *click.AppId, setVersion bool) bool {
		c.Check(setVersion, Equals, false)
		c.Check(app.Original(), Equals, appId1)
		return false
	})
	ps := &testPushService{}
	cli.pushService = ps
	cli.handleUnregister(app1)
	c.Assert(ps.unregistered, Equals, appId1)
	c.Check(cs.log.Captured(), Equals, "DEBUG unregistered token for com.example.app1_app1\n")
}

func (cs *clientSuite) TestHandleUnregisterNop(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.installedChecker = testInstalledChecker(func(app *click.AppId, setVersion bool) bool {
		c.Check(setVersion, Equals, false)
		c.Check(app.Original(), Equals, appId1)
		return true
	})
	ps := &testPushService{}
	cli.pushService = ps
	cli.handleUnregister(app1)
	c.Assert(ps.unregistered, Equals, "")
}

func (cs *clientSuite) TestHandleUnregisterError(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.installedChecker = testInstalledChecker(func(app *click.AppId, setVersion bool) bool {
		return false
	})
	fail := errors.New("BAD")
	ps := &testPushService{err: fail}
	cli.pushService = ps
	cli.handleUnregister(app1)
	c.Check(cs.log.Captured(), Matches, "ERROR unregistering com.example.app1_app1: BAD\n")
}

/*****************************************************************
    doLoop tests
******************************************************************/

var nopConn = func(bool) {}
var nopBcast = func(*session.BroadcastNotification) error { return nil }
var nopUcast = func(session.AddressedNotification) error { return nil }
var nopError = func(error) {}
var nopUnregister = func(*click.AppId) {}
var nopAcct = func() {}

func (cs *clientSuite) TestDoLoopConn(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	cli.connCh = make(chan bool, 1)
	cli.connCh <- true
	c.Assert(cli.initSessionAndPoller(), IsNil)

	ch := make(chan bool, 1)
	go cli.doLoop(func(bool) { ch <- true }, nopBcast, nopUcast, nopError, nopUnregister, nopAcct)
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopBroadcast(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSessionAndPoller(), IsNil)
	cli.session.BroadcastCh = make(chan *session.BroadcastNotification, 1)
	cli.session.BroadcastCh <- &session.BroadcastNotification{}

	ch := make(chan bool, 1)
	go cli.doLoop(nopConn, func(_ *session.BroadcastNotification) error { ch <- true; return nil }, nopUcast, nopError, nopUnregister, nopAcct)
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopNotif(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSessionAndPoller(), IsNil)
	cli.session.NotificationsCh = make(chan session.AddressedNotification, 1)
	cli.session.NotificationsCh <- session.AddressedNotification{}

	ch := make(chan bool, 1)
	go cli.doLoop(nopConn, nopBcast, func(session.AddressedNotification) error { ch <- true; return nil }, nopError, nopUnregister, nopAcct)
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopErr(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSessionAndPoller(), IsNil)
	cli.session.ErrCh = make(chan error, 1)
	cli.session.ErrCh <- nil

	ch := make(chan bool, 1)
	go cli.doLoop(nopConn, nopBcast, nopUcast, func(error) { ch <- true }, nopUnregister, nopAcct)
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopUnregister(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSessionAndPoller(), IsNil)
	cli.unregisterCh = make(chan *click.AppId, 1)
	cli.unregisterCh <- app1

	ch := make(chan bool, 1)
	go cli.doLoop(nopConn, nopBcast, nopUcast, nopError, func(app *click.AppId) { c.Check(app.Original(), Equals, appId1); ch <- true }, nopAcct)
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopAcct(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSessionAndPoller(), IsNil)
	acctCh := make(chan accounts.Changed, 1)
	acctCh <- accounts.Changed{}
	cli.accountsCh = acctCh

	ch := make(chan bool, 1)
	go cli.doLoop(nopConn, nopBcast, nopUcast, nopError, nopUnregister, func() { ch <- true })
	c.Check(takeNextBool(ch), Equals, true)
}

/*****************************************************************
    doStart tests
******************************************************************/

func (cs *clientSuite) TestDoStartWorks(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	one_called := false
	two_called := false
	one := func() error { one_called = true; return nil }
	two := func() error { two_called = true; return nil }
	c.Check(cli.doStart(one, two), IsNil)
	c.Check(one_called, Equals, true)
	c.Check(two_called, Equals, true)
}

func (cs *clientSuite) TestDoStartFailsAsExpected(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	one_called := false
	two_called := false
	failure := errors.New("Failure")
	one := func() error { one_called = true; return failure }
	two := func() error { two_called = true; return nil }
	c.Check(cli.doStart(one, two), Equals, failure)
	c.Check(one_called, Equals, true)
	c.Check(two_called, Equals, false)
}

/*****************************************************************
    Loop() tests
******************************************************************/

func (cs *clientSuite) TestLoop(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.connCh = make(chan bool)
	cli.sessionConnectedCh = make(chan uint32)
	cli.log = cs.log
	cli.connectivityEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true),
		uint32(networkmanager.ConnectedGlobal))
	cli.systemImageInfo = siInfoRes
	d := new(dumbPostal)
	cli.postalService = d
	c.Assert(cli.startPostalService(), IsNil)

	c.Assert(cli.initSessionAndPoller(), IsNil)

	cli.session.BroadcastCh = make(chan *session.BroadcastNotification)
	cli.session.ErrCh = make(chan error)

	// we use tick() to make sure things have been through the
	// event loop at least once before looking at things;
	// otherwise there's a race between what we're trying to look
	// at and the loop itself.
	tick := func() { cli.sessionConnectedCh <- 42 }

	go cli.Loop()

	// sessionConnectedCh to nothing in particular, but it'll help sync this test
	cli.sessionConnectedCh <- 42
	tick()
	c.Check(cs.log.Captured(), Matches, "(?msi).*Session connected after 42 attempts$")

	// loop() should have connected:
	//  * connCh to the connectivity checker
	c.Check(cli.hasConnectivity, Equals, false)
	cli.connCh <- true
	tick()
	c.Check(cli.hasConnectivity, Equals, true)
	cli.connCh <- false
	tick()
	c.Check(cli.hasConnectivity, Equals, false)

	//  * session.BroadcastCh to the notifications handler
	c.Check(d.bcastCount, Equals, 0)
	cli.session.BroadcastCh <- positiveBroadcastNotification
	tick()
	c.Check(d.bcastCount, Equals, 1)

	//  * session.ErrCh to the error handler
	cli.session.ErrCh <- nil
	tick()
	c.Check(cs.log.Captured(), Matches, "(?ms).*session exited.*")
}

/*****************************************************************
    Start() tests
******************************************************************/

// XXX this is a hack.
func (cs *clientSuite) hasDbus() bool {
	for _, b := range []bus.Bus{bus.SystemBus, bus.SessionBus} {
		if b.Endpoint(bus.BusDaemonAddress, cs.log).Dial() != nil {
			return false
		}
	}
	return true
}

func (cs *clientSuite) TestStart(c *C) {
	if !cs.hasDbus() {
		c.Skip("no dbus")
	}

	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	// before start, everything sucks:
	// no service,
	c.Check(cli.pushService, IsNil)
	// no config,
	c.Check(string(cli.config.Addr), Equals, "")
	// no device id,
	c.Check(cli.deviceId, HasLen, 0)
	// no session,
	c.Check(cli.session, IsNil)
	// no bus,
	c.Check(cli.systemImageEndp, IsNil)
	// no nuthin'.

	// so we start,
	err := cli.Start()
	// and it works
	c.Assert(err, IsNil)

	// and now everthing is better! We have a config,
	c.Check(string(cli.config.Addr), Equals, ":0")
	// and a device id,
	c.Check(cli.deviceId, HasLen, 40)
	// and a session,
	c.Check(cli.session, NotNil)
	// and a bus,
	c.Check(cli.systemImageEndp, NotNil)
	// and a service,
	c.Check(cli.pushService, NotNil)
	// and everthying us just peachy!
	cli.pushService.(*service.PushService).Stop() // cleanup
	cli.postalService.Stop()                      // cleanup
}

func (cs *clientSuite) TestStartCanFail(c *C) {
	cli := NewPushClient("/does/not/exist", cs.leveldbPath)
	// easiest way for it to fail is to feed it a bad config
	err := cli.Start()
	// and it works. Err. Doesn't.
	c.Check(err, NotNil)
}

func (cs *clientSuite) TestinitSessionAndPollerErr(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	// change the cli.pem value so initSessionAndPoller fails
	cli.pem = []byte("foo")
	c.Assert(cli.initSessionAndPoller(), NotNil)
}

/*****************************************************************
    getAuthorization() tests
******************************************************************/

func (cs *clientSuite) TestGetAuthorizationIgnoresErrors(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.configure()
	cli.config.AuthHelper = "/no/such/executable"

	c.Check(cli.getAuthorization("xyzzy://"), Equals, "")
}

func (cs *clientSuite) TestGetAuthorizationGetsIt(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.configure()
	cli.config.AuthHelper = helpers.ScriptAbsPath("dummyauth.sh")

	c.Check(cli.getAuthorization("xyzzy://"), Equals, "hello xyzzy://")
}

func (cs *clientSuite) TestGetAuthorizationWorksIfUnsetOrNil(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log

	c.Assert(cli.config, NotNil)
	c.Check(cli.getAuthorization("xyzzy://"), Equals, "")
	cli.configure()
	c.Check(cli.getAuthorization("xyzzy://"), Equals, "")
}
