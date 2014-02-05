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
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/bus/networkmanager"
	"launchpad.net/ubuntu-push/bus/notifications"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/client/session"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
	"launchpad.net/ubuntu-push/util"
	"launchpad.net/ubuntu-push/whoopsie/identifier"
	idtesting "launchpad.net/ubuntu-push/whoopsie/identifier/testing"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
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
	timeouts   []time.Duration
	configPath string
	log        logger.Logger
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
	cs.timeouts = util.SwapTimeouts([]time.Duration{0})
}

func (cs *clientSuite) TearDownSuite(c *C) {
	util.SwapTimeouts(cs.timeouts)
	cs.timeouts = nil
}

func (cs *clientSuite) SetUpTest(c *C) {
	cs.log = helpers.NewTestLogger(c, "debug")
	cs.log.Debugf("---")
	dir := c.MkDir()
	cs.configPath = filepath.Join(dir, "config")
	cfg := fmt.Sprintf(`
{
    "exchange_timeout": "10ms",
    "stabilizing_timeout": "0ms",
    "connectivity_check_url": "",
    "connectivity_check_md5": "",
    "addr": ":0",
    "cert_pem_file": %#v,
    "recheck_timeout": "3h",
    "log_level": "debug"
}`, helpers.SourceRelative("../server/acceptance/config/testing.cert"))
	ioutil.WriteFile(cs.configPath, []byte(cfg), 0600)
}

/*****************************************************************
    Configure tests
******************************************************************/

func (cs *clientSuite) TestConfigureWorks(c *C) {
	cli := new(Client)
	err := cli.Configure(cs.configPath)
	c.Assert(err, IsNil)
	c.Assert(cli.config, NotNil)
	c.Check(cli.config.ExchangeTimeout.Duration, Equals, time.Duration(10*time.Millisecond))
}

func (cs *clientSuite) TestConfigureSetsUpLog(c *C) {
	cli := new(Client)
	c.Check(cli.log, IsNil)
	err := cli.Configure(cs.configPath)
	c.Assert(err, IsNil)
	c.Assert(cli.log, NotNil)
}

func (cs *clientSuite) TestConfigureSetsUpPEM(c *C) {
	cli := new(Client)
	c.Check(cli.pem, IsNil)
	err := cli.Configure(cs.configPath)
	c.Assert(err, IsNil)
	c.Assert(cli.pem, NotNil)
}

func (cs *clientSuite) TestConfigureSetsUpIdder(c *C) {
	cli := new(Client)
	c.Check(cli.idder, IsNil)
	err := cli.Configure(cs.configPath)
	c.Assert(err, IsNil)
	c.Assert(cli.idder, DeepEquals, identifier.New())
}

func (cs *clientSuite) TestConfigureSetsUpEndpoints(c *C) {
	cli := new(Client)
	c.Check(cli.notificationsEndp, IsNil)
	c.Check(cli.urlDispatcherEndp, IsNil)
	c.Check(cli.connectivityEndp, IsNil)
	err := cli.Configure(cs.configPath)
	c.Assert(err, IsNil)
	c.Assert(cli.notificationsEndp, NotNil)
	c.Assert(cli.urlDispatcherEndp, NotNil)
	c.Assert(cli.connectivityEndp, NotNil)
}

func (cs *clientSuite) TestConfigureSetsUpConnCh(c *C) {
	cli := new(Client)
	c.Check(cli.connCh, IsNil)
	err := cli.Configure(cs.configPath)
	c.Assert(err, IsNil)
	c.Assert(cli.connCh, NotNil)
}

func (cs *clientSuite) TestConfigureBailsOnBadFilename(c *C) {
	cli := new(Client)
	err := cli.Configure("/does/not/exist")
	c.Assert(err, NotNil)
}

func (cs *clientSuite) TestConfigureBailsOnBadConfig(c *C) {
	cli := new(Client)
	err := cli.Configure("/etc/passwd")
	c.Assert(err, NotNil)
}

func (cs *clientSuite) TestConfigureBailsOnBadPEMFilename(c *C) {
	ioutil.WriteFile(cs.configPath, []byte(`
{
    "exchange_timeout": "10ms",
    "stabilizing_timeout": "0ms",
    "connectivity_check_url": "",
    "connectivity_check_md5": "",
    "addr": ":0",
    "cert_pem_file": "/a/b/c",
    "recheck_timeout": "3h"
}`), 0600)

	cli := new(Client)
	err := cli.Configure(cs.configPath)
	c.Assert(err, NotNil)
}

func (cs *clientSuite) TestConfigureBailsOnBadPEM(c *C) {
	ioutil.WriteFile(cs.configPath, []byte(`
{
    "exchange_timeout": "10ms",
    "stabilizing_timeout": "0ms",
    "connectivity_check_url": "",
    "connectivity_check_md5": "",
    "addr": ":0",
    "cert_pem_file": "/etc/passwd",
    "recheck_timeout": "3h"
}`), 0600)

	cli := new(Client)
	err := cli.Configure(cs.configPath)
	c.Assert(err, NotNil)
}

/*****************************************************************
    getDeviceId tests
******************************************************************/

func (cs *clientSuite) TestGetDeviceIdWorks(c *C) {
	cli := new(Client)
	cli.log = cs.log
	cli.idder = identifier.New()
	c.Check(cli.deviceId, Equals, "")
	c.Check(cli.getDeviceId(), IsNil)
	c.Check(cli.deviceId, HasLen, 128)
}

func (cs *clientSuite) TestGetDeviceIdCanFail(c *C) {
	cli := new(Client)
	cli.log = cs.log
	cli.idder = idtesting.Failing()
	c.Check(cli.deviceId, Equals, "")
	c.Check(cli.getDeviceId(), NotNil)
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
	testibus.SetWatchTicker(cEndp, make(chan bool))
	// ok, create the thing
	cli := new(Client)
	cli.log = cs.log
	err := cli.Configure(cs.configPath)
	c.Assert(err, IsNil)
	// the user actions channel has not been set up
	c.Check(cli.actionsCh, IsNil)

	// and stomp on things for testing
	cli.config.ConnectivityConfig.ConnectivityCheckURL = ts.URL
	cli.config.ConnectivityConfig.ConnectivityCheckMD5 = staticHash
	cli.notificationsEndp = nEndp
	cli.urlDispatcherEndp = uEndp
	cli.connectivityEndp = cEndp

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
}

// takeTheBus can, in fact, fail
func (cs *clientSuite) TestTakeTheBusCanFail(c *C) {
	cli := new(Client)
	err := cli.Configure(cs.configPath)
	cli.log = cs.log
	c.Assert(err, IsNil)
	// the user actions channel has not been set up
	c.Check(cli.actionsCh, IsNil)

	// and stomp on things for testing
	cli.notificationsEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))
	cli.urlDispatcherEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))
	cli.connectivityEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))

	c.Check(cli.takeTheBus(), NotNil)
	c.Check(cli.actionsCh, IsNil)
}

/*****************************************************************
    handleErr tests
******************************************************************/

func (cs *clientSuite) TestHandleErr(c *C) {
	buf := &bytes.Buffer{}
	cli := new(Client)
	cli.log = logger.NewSimpleLogger(buf, "debug")
	cli.initSession()
	cli.hasConnectivity = true
	cli.handleErr(errors.New("bananas"))
	c.Check(buf.String(), Matches, ".*session exited.*bananas\n")
}

/*****************************************************************
    handleConnState tests
******************************************************************/

func (cs *clientSuite) TestHandleConnStateD2C(c *C) {
	cli := new(Client)
	cli.log = cs.log
	cli.initSession()

	c.Assert(cli.hasConnectivity, Equals, false)
	cli.handleConnState(true)
	c.Check(cli.hasConnectivity, Equals, true)
	c.Assert(cli.session, NotNil)
}

func (cs *clientSuite) TestHandleConnStateSame(c *C) {
	cli := new(Client)
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
	cli := new(Client)
	cli.log = cs.log
	cli.session, _ = session.NewSession(string(cli.config.Addr), cli.pem, cli.config.ExchangeTimeout.Duration, cli.deviceId, cs.log)
	cli.session.Dial()
	cli.hasConnectivity = true

	// cli.session.State() will be "Error" here, for now at least
	c.Check(cli.session.State(), Not(Equals), session.Disconnected)
	cli.handleConnState(false)
	c.Check(cli.session.State(), Equals, session.Disconnected)
}

func (cs *clientSuite) TestHandleConnStateC2DPending(c *C) {
	cli := new(Client)
	cli.log = cs.log
	cli.session, _ = session.NewSession(string(cli.config.Addr), cli.pem, cli.config.ExchangeTimeout.Duration, cli.deviceId, cs.log)
	cli.hasConnectivity = true

	cli.handleConnState(false)
	c.Check(cli.session.State(), Equals, session.Disconnected)
}

/*****************************************************************
    handleNotification tests
******************************************************************/

func (cs *clientSuite) TestHandleNotification(c *C) {
	buf := &bytes.Buffer{}
	cli := new(Client)
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), uint32(1))
	cli.notificationsEndp = endp
	cli.log = logger.NewSimpleLogger(buf, "debug")
	c.Check(cli.handleNotification(), IsNil)
	// check we sent the notification
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "Notify")
	c.Check(buf.String(), Matches, `.* got notification id \d+\s*`)
}

func (cs *clientSuite) TestHandleNotificationFail(c *C) {
	cli := new(Client)
	cli.log = cs.log
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	cli.notificationsEndp = endp
	c.Check(cli.handleNotification(), NotNil)
}

/*****************************************************************
    handleClick tests
******************************************************************/

func (cs *clientSuite) TestHandleClick(c *C) {
	cli := new(Client)
	cli.log = cs.log
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), nil)
	cli.urlDispatcherEndp = endp
	c.Check(cli.handleClick(), IsNil)
	// check we sent the notification
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "DispatchURL")
	c.Check(args[0].Args, DeepEquals, []interface{}{"settings:///system/system-update"})
}

/*****************************************************************
    doLoop tests
******************************************************************/

func (cs *clientSuite) TestDoLoopConn(c *C) {
	cli := new(Client)
	cli.log = cs.log
	cli.connCh = make(chan bool, 1)
	cli.connCh <- true
	cli.initSession()

	ch := make(chan bool, 1)
	go cli.doLoop(func(bool) { ch <- true }, func() error { return nil }, func() error { return nil }, func(error) {})
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopClick(c *C) {
	cli := new(Client)
	cli.log = cs.log
	cli.initSession()
	aCh := make(chan notifications.RawActionReply, 1)
	aCh <- notifications.RawActionReply{}
	cli.actionsCh = aCh

	ch := make(chan bool, 1)
	go cli.doLoop(func(bool) {}, func() error { ch <- true; return nil }, func() error { return nil }, func(error) {})
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopNotif(c *C) {
	cli := new(Client)
	cli.log = cs.log
	cli.initSession()
	cli.session.MsgCh = make(chan *session.Notification, 1)
	cli.session.MsgCh <- &session.Notification{}

	ch := make(chan bool, 1)
	go cli.doLoop(func(bool) {}, func() error { return nil }, func() error { ch <- true; return nil }, func(error) {})
	c.Check(takeNextBool(ch), Equals, true)
}

func (cs *clientSuite) TestDoLoopErr(c *C) {
	cli := new(Client)
	cli.log = cs.log
	cli.initSession()
	cli.session.ErrCh = make(chan error, 1)
	cli.session.ErrCh <- nil

	ch := make(chan bool, 1)
	go cli.doLoop(func(bool) {}, func() error { return nil }, func() error { return nil }, func(error) { ch <- true })
	c.Check(takeNextBool(ch), Equals, true)
}

/*****************************************************************
    doStart tests
******************************************************************/

func (cs *clientSuite) TestDoStartWorks(c *C) {
	cli := new(Client)
	one_called := false
	two_called := false
	one := func() error { one_called = true; return nil }
	two := func() error { two_called = true; return nil }
	c.Check(cli.doStart(one, two), IsNil)
	c.Check(one_called, Equals, true)
	c.Check(two_called, Equals, true)
}

func (cs *clientSuite) TestDoStartFailsAsExpected(c *C) {
	cli := new(Client)
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
    loop() tests
******************************************************************/

func (cs *clientSuite) TestLoop(c *C) {
	buf := &bytes.Buffer{}
	cli := new(Client)
	cli.connCh = make(chan bool)
	cli.sessionConnectedCh = make(chan uint32)
	aCh := make(chan notifications.RawActionReply, 1)
	aCh <- notifications.RawActionReply{}
	cli.actionsCh = aCh
	cli.log = logger.NewSimpleLogger(buf, "debug")
	cli.notificationsEndp = testibus.NewMultiValuedTestingEndpoint(condition.Work(true),
		condition.Work(true), []interface{}{uint32(1), "hello"})
	cli.urlDispatcherEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))
	cli.connectivityEndp = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true),
		uint32(networkmanager.ConnectedGlobal))

	cli.initSession()

	cli.session.MsgCh = make(chan *session.Notification)
	cli.session.ErrCh = make(chan error)

	go cli.loop()

	// sessionConnectedCh to nothing in particular, but it'll help sync this test
	cli.sessionConnectedCh <- 42
	cli.sessionConnectedCh <- 42 // we check things one loop later (loopy, i know)
	c.Check(buf, Matches, "(?ms).*Session connected after 42 attempts$")

	//  * actionsCh to the click handler/url dispatcher
	uargs := testibus.GetCallArgs(cli.urlDispatcherEndp)
	c.Assert(uargs, HasLen, 1)
	c.Check(uargs[0].Member, Equals, "DispatchURL")

	// loop() should have connected:
	//  * connCh to the connectivity checker
	c.Check(cli.hasConnectivity, Equals, false)
	cli.connCh <- true
	cli.sessionConnectedCh <- 42
	c.Check(cli.hasConnectivity, Equals, true)
	cli.connCh <- false
	cli.sessionConnectedCh <- 42
	c.Check(cli.hasConnectivity, Equals, false)

	//  * session.MsgCh to the notifications handler
	cli.session.MsgCh <- &session.Notification{}
	cli.sessionConnectedCh <- 42
	nargs := testibus.GetCallArgs(cli.notificationsEndp)
	c.Check(nargs, HasLen, 1)

	//  * session.ErrCh to the error handler
	cli.session.ErrCh <- nil
	cli.sessionConnectedCh <- 42
	c.Check(buf, Matches, "(?ms).*session exited.*")
}
