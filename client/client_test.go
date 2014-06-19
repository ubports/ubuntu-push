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
	"testing"
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/networkmanager"
	"launchpad.net/ubuntu-push/bus/notifications"
	"launchpad.net/ubuntu-push/bus/systemimage"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/client/session"
	"launchpad.net/ubuntu-push/client/session/seenstate"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/protocol"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
	"launchpad.net/ubuntu-push/util"
	"launchpad.net/ubuntu-push/whoopsie/identifier"
	idtesting "launchpad.net/ubuntu-push/whoopsie/identifier/testing"
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
	cs.timeouts = util.SwapTimeouts([]time.Duration{0})
	cs.leveldbPath = ""
}

func (cs *clientSuite) TearDownSuite(c *C) {
	util.SwapTimeouts(cs.timeouts)
	cs.timeouts = nil
}

func (cs *clientSuite) writeTestConfig(overrides map[string]interface{}) {
	pem_file := helpers.SourceRelative("../server/acceptance/ssl/testing.cert")
	cfgMap := map[string]interface{}{
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
	c.Assert(cli.idder, FitsTypeOf, identifier.New())
}

func (cs *clientSuite) TestConfigureSetsUpEndpoints(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	c.Check(cli.notificationsEndp, IsNil)
	c.Check(cli.urlDispatcherEndp, IsNil)
	c.Check(cli.connectivityEndp, IsNil)
	err := cli.configure()
	c.Assert(err, IsNil)
	c.Assert(cli.notificationsEndp, NotNil)
	c.Assert(cli.urlDispatcherEndp, NotNil)
	c.Assert(cli.connectivityEndp, NotNil)
}

func (cs *clientSuite) TestConfigureSetsUpConnCh(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	c.Check(cli.connCh, IsNil)
	err := cli.configure()
	c.Assert(err, IsNil)
	c.Assert(cli.connCh, NotNil)
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
		PEM:        cli.pem,
		Info:       info,
		AuthGetter: func(string) string { return "" },
		AuthURL:    "xyzzy://",
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
	c.Check(fmt.Sprintf("%v", conf.AuthGetter), Equals, fmt.Sprintf("%v", cli.getAuthorization))
	// and set it to nil
	conf.AuthGetter = nil
	expected.AuthGetter = nil
	c.Check(conf, DeepEquals, expected)
}

/*****************************************************************
    startService tests
******************************************************************/

func (cs *clientSuite) TestStartServiceWorks(c *C) {
	cs.writeTestConfig(map[string]interface{}{
		"auth_helper": "../scripts/dummyauth.sh",
	})
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.configure()
	cli.log = cs.log
	cli.deviceId = "fake-id"
	cli.serviceEndpoint = testibus.NewTestingEndpoint(condition.Work(true), nil)
	c.Check(cli.service, IsNil)
	c.Check(cli.startService(), IsNil)
	c.Assert(cli.service, NotNil)
	c.Check(cli.service.IsRunning(), Equals, true)
	c.Check(cli.service.GetMessageHandler(), NotNil)
	c.Check(cli.service.GetRegistrationAuthorization(), Equals, "hello reg://")
	c.Check(cli.service.GetDeviceId(), Equals, "fake-id")
	cli.service.Stop()
}

func (cs *clientSuite) TestStartServiceErrorsOnNilLog(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	c.Check(cli.log, IsNil)
	c.Check(cli.startService(), NotNil)
}

func (cs *clientSuite) TestStartServiceErrorsOnBusDialFail(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.serviceEndpoint = testibus.NewTestingEndpoint(condition.Work(false), nil)
	c.Check(cli.startService(), NotNil)
}

/*****************************************************************
    getDeviceId tests
******************************************************************/

func (cs *clientSuite) TestGetDeviceIdWorks(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.idder = identifier.New()
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

func (cs *clientSuite) TestGetDeviceIdWhoopsieDoesTheUnexpected(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	settable := idtesting.Settable()
	cli.idder = settable
	settable.Set("not-hex")
	c.Check(cli.deviceId, Equals, "")
	c.Check(cli.getDeviceId(), ErrorMatches, "whoopsie id should be hex: .*")
}

/*****************************************************************
    takeTheBus tests
******************************************************************/

func (cs *clientSuite) TestTakeTheBusWorks(c *C) {
	// http server used for connectivity test
	ts := httptest.NewServer(mkHandler(staticText))
	defer ts.Close()

	// testing endpoints
	nCond := condition.Fail2Work(3)
	nEndp := testibus.NewMultiValuedTestingEndpoint(nCond, condition.Work(true),
		[]interface{}{uint32(1), "hello"})
	uCond := condition.Fail2Work(5)
	uEndp := testibus.NewTestingEndpoint(uCond, condition.Work(false))
	cCond := condition.Fail2Work(7)
	cEndp := testibus.NewTestingEndpoint(cCond, condition.Work(true),
		uint32(networkmanager.ConnectedGlobal),
	)
	siCond := condition.Fail2Work(2)
	siEndp := testibus.NewMultiValuedTestingEndpoint(siCond, condition.Work(true), []interface{}{int32(101), "mako", "daily", "Unknown", map[string]string{}})
	testibus.SetWatchTicker(cEndp, make(chan bool))
	// ok, create the thing
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	err := cli.configure()
	c.Assert(err, IsNil)
	// the user actions channel has not been set up
	c.Check(cli.actionsCh, IsNil)

	// and stomp on things for testing
	cli.config.ConnectivityConfig.ConnectivityCheckURL = ts.URL
	cli.config.ConnectivityConfig.ConnectivityCheckMD5 = staticHash
	cli.notificationsEndp = nEndp
	cli.urlDispatcherEndp = uEndp
	cli.connectivityEndp = cEndp
	cli.systemImageEndp = siEndp

	c.Assert(cli.takeTheBus(), IsNil)
	// the notifications and urldispatcher endpoints retried until connected
	c.Check(nCond.OK(), Equals, true)
	c.Check(uCond.OK(), Equals, true)
	// the user actions channel has now been set up
	c.Check(cli.actionsCh, NotNil)
	c.Check(takeNextBool(cli.connCh), Equals, false)
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
	// the user actions channel has not been set up
	c.Check(cli.actionsCh, IsNil)

	// and stomp on things for testing
	cli.notificationsEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))
	cli.urlDispatcherEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))
	cli.connectivityEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))
	cli.systemImageEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))

	c.Check(cli.takeTheBus(), NotNil)
	c.Check(cli.actionsCh, IsNil)
}

/*****************************************************************
    handleErr tests
******************************************************************/

func (cs *clientSuite) TestHandleErr(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSession(), IsNil)
	cs.log.ResetCapture()
	cli.hasConnectivity = true
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
	c.Check(fmt.Sprintf("%T", ln), Equals, "*seenstate.memSeenState")
}

func (cs *clientSuite) TestSeenStateFactoryWithDbPath(c *C) {
	cli := NewPushClient(cs.configPath, ":memory:")
	ln, err := cli.seenStateFactory()
	c.Assert(err, IsNil)
	c.Check(fmt.Sprintf("%T", ln), Equals, "*seenstate.sqliteSeenState")
}

/*****************************************************************
    handleConnState tests
******************************************************************/

func (cs *clientSuite) TestHandleConnStateD2C(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSession(), IsNil)

	c.Assert(cli.hasConnectivity, Equals, false)
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
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), uint32(1))
	cli.notificationsEndp = endp
	cli.log = cs.log
	c.Check(cli.handleBroadcastNotification(positiveBroadcastNotification), IsNil)
	// check we sent the notification
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "Notify")
	c.Check(cs.log.Captured(), Matches, `.* got notification id \d+\s*`)
}

func (cs *clientSuite) TestHandleBroadcastNotificationNothingToDo(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.systemImageInfo = siInfoRes
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), uint32(1))
	cli.notificationsEndp = endp
	cli.log = cs.log
	c.Check(cli.handleBroadcastNotification(negativeBroadcastNotification), IsNil)
	// check we sent the notification
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 0)
	c.Check(cs.log.Captured(), Matches, "")
}

func (cs *clientSuite) TestHandleBroadcastNotificationFail(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.systemImageInfo = siInfoRes
	cli.log = cs.log
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	cli.notificationsEndp = endp
	c.Check(cli.handleBroadcastNotification(positiveBroadcastNotification), NotNil)
}

/*****************************************************************
    handleUnicastNotification tests
******************************************************************/

var notif = &protocol.Notification{AppId: "hello", Payload: []byte(`{"url": "xyzzy"}`), MsgId: "42"}

func (cs *clientSuite) TestHandleUcastNotification(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	svcEndp := testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), uint32(1))
	cli.log = cs.log
	cli.serviceEndpoint = svcEndp
	notsEndp := testibus.NewTestingEndpoint(nil, condition.Work(true), uint32(1))
	cli.notificationsEndp = notsEndp
	c.Assert(cli.startService(), IsNil)
	c.Check(cli.handleUnicastNotification(notif), IsNil)
	// check we sent the notification
	args := testibus.GetCallArgs(svcEndp)
	c.Assert(len(args), Not(Equals), 0)
	c.Check(args[len(args)-1].Member, Equals, "::Signal")
	c.Check(cs.log.Captured(), Matches, `(?m).*sending notification "42" for "hello".*`)
}

/*****************************************************************
    handleClick tests
******************************************************************/

func (cs *clientSuite) TestHandleClick(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))
	cli.urlDispatcherEndp = endp
	// check we don't fail on something random
	c.Check(cli.handleClick("something random"), IsNil)
	// ... but we don't send anything either
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 0)
	// check we worked with the right action id
	c.Check(cli.handleClick(ACTION_ID_BROADCAST), IsNil)
	// check we sent the notification
	args = testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "DispatchURL")
	c.Check(args[0].Args, DeepEquals, []interface{}{system_update_url})
	// check we worked with the right action id
	c.Check(cli.handleClick(ACTION_ID_SNOWFLAKE+"foo"), IsNil)
	// check we sent the notification
	args = testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 2)
	c.Check(args[1].Member, Equals, "DispatchURL")
	c.Check(args[1].Args, DeepEquals, []interface{}{"foo"})
}

/*****************************************************************
    doLoop tests
******************************************************************/

var nopConn = func(bool) {}
var nopClick = func(string) error { return nil }
var nopBcast = func(*session.BroadcastNotification) error { return nil }
var nopUcast = func(*protocol.Notification) error { return nil }
var nopError = func(error) {}

func (cs *clientSuite) TestDoLoopConn(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	cli.connCh = make(chan bool, 1)
	cli.connCh <- true
	c.Assert(cli.initSession(), IsNil)

	ch := make(chan bool, 1)
	go cli.doLoop(func(bool) { ch <- true }, nopClick, nopBcast, nopUcast, nopError)
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopClick(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSession(), IsNil)
	aCh := make(chan notifications.RawActionReply, 1)
	aCh <- notifications.RawActionReply{}
	cli.actionsCh = aCh

	ch := make(chan bool, 1)
	go cli.doLoop(nopConn, func(_ string) error { ch <- true; return nil }, nopBcast, nopUcast, nopError)
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopBroadcast(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSession(), IsNil)
	cli.session.BroadcastCh = make(chan *session.BroadcastNotification, 1)
	cli.session.BroadcastCh <- &session.BroadcastNotification{}

	ch := make(chan bool, 1)
	go cli.doLoop(nopConn, nopClick, func(_ *session.BroadcastNotification) error { ch <- true; return nil }, nopUcast, nopError)
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopNotif(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSession(), IsNil)
	cli.session.NotificationsCh = make(chan *protocol.Notification, 1)
	cli.session.NotificationsCh <- &protocol.Notification{}

	ch := make(chan bool, 1)
	go cli.doLoop(nopConn, nopClick, nopBcast, func(*protocol.Notification) error { ch <- true; return nil }, nopError)
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopErr(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSession(), IsNil)
	cli.session.ErrCh = make(chan error, 1)
	cli.session.ErrCh <- nil

	ch := make(chan bool, 1)
	go cli.doLoop(nopConn, nopClick, nopBcast, nopUcast, func(error) { ch <- true })
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
	aCh := make(chan notifications.RawActionReply, 1)
	cli.actionsCh = aCh
	cli.log = cs.log
	cli.notificationsEndp = testibus.NewMultiValuedTestingEndpoint(condition.Work(true),
		condition.Work(true), []interface{}{uint32(1), "hello"})
	cli.urlDispatcherEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))
	cli.connectivityEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true),
		uint32(networkmanager.ConnectedGlobal))
	cli.systemImageInfo = siInfoRes
	c.Assert(cli.initSession(), IsNil)

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
	c.Check(cs.log.Captured(), Matches, "(?ms).*Session connected after 42 attempts$")

	//  * actionsCh to the click handler/url dispatcher
	aCh <- notifications.RawActionReply{ActionId: ACTION_ID_BROADCAST}
	tick()
	uargs := testibus.GetCallArgs(cli.urlDispatcherEndp)
	c.Assert(uargs, HasLen, 1)
	c.Check(uargs[0].Member, Equals, "DispatchURL")

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
	cli.session.BroadcastCh <- positiveBroadcastNotification
	tick()
	nargs := testibus.GetCallArgs(cli.notificationsEndp)
	c.Check(nargs, HasLen, 1)

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
	c.Check(cli.service, IsNil)
	// no config,
	c.Check(string(cli.config.Addr), Equals, "")
	// no device id,
	c.Check(cli.deviceId, HasLen, 0)
	// no session,
	c.Check(cli.session, IsNil)
	// no bus,
	c.Check(cli.notificationsEndp, IsNil)
	// no nuthin'.

	// so we start,
	err := cli.Start()
	// and it works
	c.Check(err, IsNil)

	// and now everthing is better! We have a config,
	c.Check(string(cli.config.Addr), Equals, ":0")
	// and a device id,
	c.Check(cli.deviceId, HasLen, 40)
	// and a session,
	c.Check(cli.session, NotNil)
	// and a bus,
	c.Check(cli.notificationsEndp, NotNil)
	// and a service,
	c.Check(cli.service, NotNil)
	// and everthying us just peachy!
	cli.service.Stop() // cleanup
}

func (cs *clientSuite) TestStartCanFail(c *C) {
	cli := NewPushClient("/does/not/exist", cs.leveldbPath)
	// easiest way for it to fail is to feed it a bad config
	err := cli.Start()
	// and it works. Err. Doesn't.
	c.Check(err, NotNil)
}

func (cs *clientSuite) TestMessageHandler(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), uint32(1))
	cli.notificationsEndp = endp
	cli.log = cs.log
	err := cli.messageHandler([]byte(`{"icon": "icon-value", "summary": "summary-value", "body": "body-value"}`))
	c.Assert(err, IsNil)
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "Notify")
	c.Check(args[0].Args[0], Equals, "ubuntu-push-client")
	c.Check(args[0].Args[2], Equals, "icon-value")
	c.Check(args[0].Args[3], Equals, "summary-value")
	c.Check(args[0].Args[4], Equals, "body-value")
}

func (cs *clientSuite) TestMessageHandlerReportsUnmarshalErrors(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	cli.log = cs.log

	err := cli.messageHandler([]byte(`{"broken`))
	c.Check(err, NotNil)
	c.Check(cs.log.Captured(), Matches, "(?msi).*unable to unmarshal message:.*")
}

func (cs *clientSuite) TestMessageHandlerReportsFailedNotifies(c *C) {
	cli := NewPushClient(cs.configPath, cs.leveldbPath)
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	cli.notificationsEndp = endp
	cli.log = cs.log
	err := cli.messageHandler([]byte(`{}`))
	c.Assert(err, NotNil)
	c.Check(cs.log.Captured(), Matches, "(?msi).*showing notification: no way$")
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
	cli.config.AuthHelper = "../scripts/dummyauth.sh"

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
