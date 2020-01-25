/*
 Copyright 2014 Canonical Ltd.

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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"launchpad.net/go-dbus"
	. "launchpad.net/gocheck"

	"github.com/ubports/ubuntu-push/bus"
	"github.com/ubports/ubuntu-push/bus/notifications"
	testibus "github.com/ubports/ubuntu-push/bus/testing"
	"github.com/ubports/ubuntu-push/bus/windowstack"
	"github.com/ubports/ubuntu-push/click"
	clickhelp "github.com/ubports/ubuntu-push/click/testing"
	"github.com/ubports/ubuntu-push/launch_helper"
	"github.com/ubports/ubuntu-push/messaging/reply"
	"github.com/ubports/ubuntu-push/nih"
	helpers "github.com/ubports/ubuntu-push/testing"
	"github.com/ubports/ubuntu-push/testing/condition"
)

// takeNext takes a value from given channel with a 5s timeout
func takeNextBool(ch <-chan bool) bool {
	select {
	case <-time.After(5 * time.Second):
		panic("channel stuck: too long waiting")
	case v := <-ch:
		return v
	}
}

// takeNextBytes takes a value from given channel with a 5s timeout
func takeNextBytes(ch <-chan []byte) []byte {
	select {
	case <-time.After(5 * time.Second):
		panic("channel stuck: too long waiting")
	case v := <-ch:
		return v
	}
}

// takeNextHelperOutput takes a value from given channel with a 5s timeout
func takeNextHelperOutput(ch <-chan *launch_helper.HelperOutput) *launch_helper.HelperOutput {
	select {
	case <-time.After(5 * time.Second):
		panic("channel stuck: too long waiting")
	case v := <-ch:
		return v
	}
}

func takeNextError(ch <-chan error) error {
	select {
	case <-time.After(5 * time.Second):
		panic("channel stuck: too long waiting")
	case v := <-ch:
		return v
	}
}

func installTickMessageHandler(svc *PostalService) chan bool {
	ch := make(chan bool)
	msgHandler := svc.GetMessageHandler()
	svc.SetMessageHandler(func(app *click.AppId, nid string, output *launch_helper.HelperOutput) bool {
		var b bool
		if msgHandler != nil {
			b = msgHandler(app, nid, output)
		}
		ch <- b
		return b
	})
	return ch
}

type fakeHelperLauncher struct {
	i    int
	ch   chan []byte
	done func(string)
}

func (fhl *fakeHelperLauncher) InstallObserver(done func(string)) error {
	fhl.done = done
	return nil
}
func (fhl *fakeHelperLauncher) RemoveObserver() error  { return nil }
func (fhl *fakeHelperLauncher) Stop(_, _ string) error { return nil }
func (fhl *fakeHelperLauncher) HelperInfo(app *click.AppId) (string, string) {
	if app.Click {
		return "helpId", "bar"
	} else {
		return "", "lhex"
	}
}
func (fhl *fakeHelperLauncher) Launch(_, _, f1, f2 string) (string, error) {
	dat, err := ioutil.ReadFile(f1)
	if err != nil {
		return "", err
	}
	err = ioutil.WriteFile(f2, dat, os.ModeTemporary)
	if err != nil {
		return "", err
	}

	id := []string{"0", "1", "2"}[fhl.i]
	fhl.i++

	fhl.ch <- dat

	return id, nil
}

type fakeUrlDispatcher struct {
	DispatchDone       chan bool
	DispatchCalls      [][]string
	TestURLCalls       []map[string][]string
	NextTestURLResult  bool
	DispatchShouldFail bool
	Lock               sync.Mutex
}

func (fud *fakeUrlDispatcher) DispatchURL(url string, app *click.AppId) error {
	fud.Lock.Lock()
	defer fud.Lock.Unlock()
	fud.DispatchCalls = append(fud.DispatchCalls, []string{url, app.DispatchPackage()})
	if fud.DispatchShouldFail {
		return errors.New("fail!")
	}
	fud.DispatchDone <- true
	return nil
}

func (fud *fakeUrlDispatcher) TestURL(app *click.AppId, urls []string) bool {
	fud.Lock.Lock()
	defer fud.Lock.Unlock()
	var args = make(map[string][]string, 1)
	args[app.DispatchPackage()] = urls
	fud.TestURLCalls = append(fud.TestURLCalls, args)
	return fud.NextTestURLResult
}

type postalSuite struct {
	log             *helpers.TestLogger
	cfg             *PostalServiceSetup
	bus             bus.Endpoint
	notifBus        bus.Endpoint
	counterBus      bus.Endpoint
	hapticBus       bus.Endpoint
	unityGreeterBus bus.Endpoint
	winStackBus     bus.Endpoint
	accountsBus     bus.Endpoint
	accountsCh      chan []interface{}
	fakeLauncher    *fakeHelperLauncher
	getTempDir      func(string) (string, error)
	oldAreEnabled   func(*click.AppId) bool
	notifyEnabled   bool
}

type ualPostalSuite struct {
	postalSuite
}

type trivialPostalSuite struct {
	postalSuite
}

var _ = Suite(&ualPostalSuite{})
var _ = Suite(&trivialPostalSuite{})

func (ps *postalSuite) SetUpTest(c *C) {
	ps.oldAreEnabled = areNotificationsEnabled
	areNotificationsEnabled = func(*click.AppId) bool { return ps.notifyEnabled }
	ps.log = helpers.NewTestLogger(c, "debug")
	ps.cfg = &PostalServiceSetup{}
	ps.bus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true))
	ps.notifBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true))
	ps.counterBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true))
	ps.accountsBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), map[string]dbus.Variant{
		"IncomingMessageVibrate":           dbus.Variant{true},
		"SilentMode":                       dbus.Variant{false},
		"IncomingMessageSound":             dbus.Variant{""},
		"IncomingMessageVibrateSilentMode": dbus.Variant{false},
	})

	ps.hapticBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true))
	ps.unityGreeterBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), false)
	ps.winStackBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), []windowstack.WindowsInfo{})
	ps.fakeLauncher = &fakeHelperLauncher{ch: make(chan []byte)}
	ps.notifyEnabled = true

	ps.getTempDir = launch_helper.GetTempDir
	d := c.MkDir()
	launch_helper.GetTempDir = func(pkgName string) (string, error) {
		tmpDir := filepath.Join(d, pkgName)
		return tmpDir, os.MkdirAll(tmpDir, 0700)
	}

	ps.accountsCh = make(chan []interface{})
	testibus.SetWatchSource(ps.accountsBus, "PropertiesChanged", ps.accountsCh)
}

func (ps *postalSuite) TearDownTest(c *C) {
	areNotificationsEnabled = ps.oldAreEnabled
	launch_helper.GetTempDir = ps.getTempDir
	close(ps.accountsCh)
}

func (ts *trivialPostalSuite) SetUpTest(c *C) {
	ts.postalSuite.SetUpTest(c)
	useTrivialHelper = true
}

func (ts *trivialPostalSuite) TearDownTest(c *C) {
	ts.postalSuite.TearDownTest(c)
	useTrivialHelper = false
}

func (ps *postalSuite) replaceBuses(pst *PostalService) *PostalService {
	pst.Bus = ps.bus
	pst.NotificationsEndp = ps.notifBus
	pst.EmblemCounterEndp = ps.counterBus
	pst.AccountsEndp = ps.accountsBus
	pst.HapticEndp = ps.hapticBus
	pst.UnityGreeterEndp = ps.unityGreeterBus
	pst.WindowStackEndp = ps.winStackBus
	pst.launchers = map[string]launch_helper.HelperLauncher{}
	return pst
}

func (ps *postalSuite) TestStart(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	c.Check(svc.IsRunning(), Equals, false)
	c.Check(svc.Start(), IsNil)
	c.Check(svc.IsRunning(), Equals, true)
	svc.Stop()
}

func (ps *postalSuite) TestStartTwice(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	c.Check(svc.Start(), IsNil)
	c.Check(svc.Start(), Equals, ErrAlreadyStarted)
	svc.Stop()
}

func (ps *postalSuite) TestStartNoLog(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, nil))
	c.Check(svc.Start(), Equals, ErrNotConfigured)
}

func (ps *postalSuite) TestStartNoBus(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.Bus = nil
	c.Check(svc.Start(), Equals, ErrNotConfigured)

	svc = ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.NotificationsEndp = nil
	c.Check(svc.Start(), Equals, ErrNotConfigured)
}

func (ps *postalSuite) TestTakeTheBusFail(c *C) {
	nEndp := testibus.NewMultiValuedTestingEndpoint(condition.Work(true), condition.Work(false))
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.NotificationsEndp = nEndp
	_, err := svc.takeTheBus()
	c.Check(err, NotNil)
}

func (ps *postalSuite) TestTakeTheBusOk(c *C) {
	nEndp := testibus.NewMultiValuedTestingEndpoint(condition.Work(true), condition.Work(true), []interface{}{uint32(1), "hello"})
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.NotificationsEndp = nEndp
	_, err := svc.takeTheBus()
	c.Check(err, IsNil)
}

func (ps *postalSuite) TestStartFailsOnBusDialFailure(c *C) {
	// XXX actually, we probably want to autoredial this
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.Bus = testibus.NewTestingEndpoint(condition.Work(false), nil)
	c.Check(svc.Start(), ErrorMatches, `.*(?i)cond said no.*`)
	svc.Stop()
}

func (ps *postalSuite) TestStartGrabsName(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	c.Assert(svc.Start(), IsNil)
	callArgs := testibus.GetCallArgs(ps.bus)
	defer svc.Stop()
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[0].Member, Equals, "::GrabName")
}

func (ps *postalSuite) TestStopClosesBus(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	c.Assert(svc.Start(), IsNil)
	svc.Stop()
	callArgs := testibus.GetCallArgs(ps.bus)
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[len(callArgs)-1].Member, Equals, "::Close")
}

//
// post() tests

func (ps *postalSuite) TestPostHappyPath(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.msgHandler = nil
	ch := installTickMessageHandler(svc)
	svc.launchers = map[string]launch_helper.HelperLauncher{
		"click": ps.fakeLauncher,
	}
	c.Assert(svc.Start(), IsNil)
	payload := `{"message": {"world":1}}`
	rvs, err := svc.post(aPackageOnBus, []interface{}{anAppId, payload}, nil)
	c.Assert(err, IsNil)
	c.Check(rvs, IsNil)

	if ps.fakeLauncher.done != nil {
		// wait for the two posts to "launch"
		inputData := takeNextBytes(ps.fakeLauncher.ch)
		c.Check(string(inputData), Equals, payload)

		go ps.fakeLauncher.done("0") // OneDone
	}

	c.Check(takeNextBool(ch), Equals, false) // one,
	// xxx here?
	c.Assert(svc.mbox, HasLen, 1)
	box, ok := svc.mbox[anAppId]
	c.Check(ok, Equals, true)
	msgs := box.AllMessages()
	c.Assert(msgs, HasLen, 1)
	c.Check(msgs[0], Equals, `{"world":1}`)
	c.Check(box.nids[0], Not(Equals), "")
}

func (ps *postalSuite) TestPostFailsIfBadArgs(c *C) {
	for i, s := range []struct {
		args []interface{}
		errt error
	}{
		{nil, ErrBadArgCount},
		{[]interface{}{}, ErrBadArgCount},
		{[]interface{}{1}, ErrBadArgCount},
		{[]interface{}{anAppId, 1}, ErrBadArgType},
		{[]interface{}{anAppId, "zoom"}, ErrBadJSON},
		{[]interface{}{1, "hello"}, ErrBadArgType},
		{[]interface{}{1, 2, 3}, ErrBadArgCount},
		{[]interface{}{"bar", "hello"}, click.ErrInvalidAppId},
	} {
		reg, err := new(PostalService).post(aPackageOnBus, s.args, nil)
		c.Check(reg, IsNil, Commentf("iteration #%d", i))
		c.Check(err, Equals, s.errt, Commentf("iteration #%d", i))
	}
}

// Post() tests

func (ps *postalSuite) TestPostWorks(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.msgHandler = nil
	ch := installTickMessageHandler(svc)
	fakeLauncher2 := &fakeHelperLauncher{ch: make(chan []byte)}
	svc.launchers = map[string]launch_helper.HelperLauncher{
		"click":  ps.fakeLauncher,
		"legacy": fakeLauncher2,
	}
	c.Assert(svc.Start(), IsNil)

	app := clickhelp.MustParseAppId(anAppId)
	// these two, being for the same app, will be done sequentially.
	svc.Post(app, "m1", json.RawMessage(`{"message":{"world":1}}`))
	svc.Post(app, "m2", json.RawMessage(`{"message":{"moon":1}}`))
	classicApp := clickhelp.MustParseAppId("_classic-app")
	svc.Post(classicApp, "m3", json.RawMessage(`{"message":{"mars":42}}`))

	oneConsumed := false

	if ps.fakeLauncher.done != nil {
		// wait for the two posts to "launch"
		takeNextBytes(ps.fakeLauncher.ch)
		takeNextBytes(fakeLauncher2.ch)
		go ps.fakeLauncher.done("0") // OneDone
		go fakeLauncher2.done("0")

		c.Check(takeNextBool(ch), Equals, false) // one
		oneConsumed = true

		inputData := takeNextBytes(ps.fakeLauncher.ch)

		c.Check(string(inputData), Equals, `{"message":{"moon":1}}`)

		go ps.fakeLauncher.done("1") // OneDone
	}

	if !oneConsumed {
		c.Check(takeNextBool(ch), Equals, false) // one,
	}
	c.Check(takeNextBool(ch), Equals, false) // two,
	c.Check(takeNextBool(ch), Equals, false) // three posts
	c.Assert(svc.mbox, HasLen, 2)
	box, ok := svc.mbox[anAppId]
	c.Check(ok, Equals, true)
	msgs := box.AllMessages()
	c.Assert(msgs, HasLen, 2)
	c.Check(msgs[0], Equals, `{"world":1}`)
	c.Check(msgs[1], Equals, `{"moon":1}`)
	c.Check(box.nids, DeepEquals, []string{"m1", "m2"})
	box, ok = svc.mbox["_classic-app"]
	c.Assert(ok, Equals, true)
	msgs = box.AllMessages()
	c.Assert(msgs, HasLen, 1)
	c.Check(msgs[0], Equals, `{"mars":42}`)
}

func (ps *postalSuite) TestPostCallsMessageHandlerDetails(c *C) {
	ch := make(chan *launch_helper.HelperOutput)
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.launchers = map[string]launch_helper.HelperLauncher{
		"click": ps.fakeLauncher,
	}
	c.Assert(svc.Start(), IsNil)
	// check the message handler gets called
	app := clickhelp.MustParseAppId(anAppId)
	f := func(app *click.AppId, nid string, s *launch_helper.HelperOutput) bool {
		c.Check(app.Base(), Equals, anAppId)
		c.Check(nid, Equals, "m7")
		ch <- s
		return true
	}
	svc.SetMessageHandler(f)
	svc.Post(app, "m7", json.RawMessage("{}"))

	if ps.fakeLauncher.done != nil {
		takeNextBytes(ps.fakeLauncher.ch)

		go ps.fakeLauncher.done("0") // OneDone
	}

	c.Check(takeNextHelperOutput(ch), DeepEquals, &launch_helper.HelperOutput{})
}

func (ps *postalSuite) TestAfterMessageHandlerSignal(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.msgHandler = nil

	hInp := &launch_helper.HelperInput{
		App: clickhelp.MustParseAppId(anAppId),
	}
	res := &launch_helper.HelperResult{Input: hInp}

	svc.handleHelperResult(res)

	// and check it fired the right signal
	callArgs := testibus.GetCallArgs(ps.bus)
	l := len(callArgs)
	if l < 1 {
		c.Fatal("not enough elements in resposne from GetCallArgs")
	}
	c.Check(callArgs[l-1].Member, Equals, "::Signal")
	c.Check(callArgs[l-1].Args, DeepEquals, []interface{}{"Post", aPackageOnBus, []interface{}{anAppId}})
}

func (ps *postalSuite) TestFailingMessageHandlerSurvived(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.SetMessageHandler(func(*click.AppId, string, *launch_helper.HelperOutput) bool {
		return false
	})

	hInp := &launch_helper.HelperInput{
		App: clickhelp.MustParseAppId(anAppId),
	}
	res := &launch_helper.HelperResult{Input: hInp}

	svc.handleHelperResult(res)

	c.Check(ps.log.Captured(), Equals, "DEBUG msgHandler did not present the notification\n")
	// we actually want to send a signal even if we didn't do anything
	callArgs := testibus.GetCallArgs(ps.bus)
	c.Assert(len(callArgs), Equals, 1)
	c.Check(callArgs[0].Member, Equals, "::Signal")
}

//
// Notifications tests
func (ps *postalSuite) TestNotificationsWorks(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	nots, err := svc.popAll(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Assert(err, IsNil)
	c.Assert(nots, NotNil)
	c.Assert(nots, HasLen, 1)
	c.Check(nots[0], HasLen, 0)
	c.Assert(svc.mbox, IsNil)
	svc.mbox = make(map[string]*mBox)
	nots, err = svc.popAll(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Assert(err, IsNil)
	c.Assert(nots, NotNil)
	c.Assert(nots, HasLen, 1)
	c.Check(nots[0], HasLen, 0)
	box := new(mBox)
	svc.mbox[anAppId] = box
	m1 := json.RawMessage(`"m1"`)
	m2 := json.RawMessage(`"m2"`)
	box.Append(m1, "n1")
	box.Append(m2, "n2")
	nots, err = svc.popAll(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Assert(err, IsNil)
	c.Assert(nots, NotNil)
	c.Assert(nots, HasLen, 1)
	c.Check(nots[0], DeepEquals, []string{string(m1), string(m2)})
}

func (ps *postalSuite) TestNotificationsFailsIfBadArgs(c *C) {
	for i, s := range []struct {
		args []interface{}
		errt error
	}{
		{nil, ErrBadArgCount},
		{[]interface{}{}, ErrBadArgCount},
		{[]interface{}{1}, ErrBadArgType},
		{[]interface{}{"potato"}, click.ErrInvalidAppId},
	} {
		reg, err := new(PostalService).popAll(aPackageOnBus, s.args, nil)
		c.Check(reg, IsNil, Commentf("iteration #%d", i))
		c.Check(err, Equals, s.errt, Commentf("iteration #%d", i))
	}
}

func (ps *postalSuite) TestMessageHandlerPublicAPI(c *C) {
	svc := new(PostalService)
	c.Assert(svc.msgHandler, IsNil)
	var ext = &launch_helper.HelperOutput{}
	f := func(_ *click.AppId, _ string, s *launch_helper.HelperOutput) bool { ext = s; return false }
	c.Check(svc.GetMessageHandler(), IsNil)
	svc.SetMessageHandler(f)
	c.Check(svc.GetMessageHandler(), NotNil)
	hOutput := &launch_helper.HelperOutput{[]byte("37"), nil}
	c.Check(svc.msgHandler(nil, "", hOutput), Equals, false)
	c.Check(ext, DeepEquals, hOutput)
}

func (ps *postalSuite) TestMessageHandlerPresents(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), uint32(1))
	svc := NewPostalService(ps.cfg, ps.log)
	svc.Bus = endp
	svc.EmblemCounterEndp = endp
	svc.AccountsEndp = ps.accountsBus
	svc.HapticEndp = endp
	svc.NotificationsEndp = endp
	svc.UnityGreeterEndp = ps.unityGreeterBus
	svc.WindowStackEndp = ps.winStackBus
	nopTicker := make(chan []interface{})
	testibus.SetWatchSource(endp, "ActionInvoked", nopTicker)
	defer close(nopTicker)
	svc.launchers = map[string]launch_helper.HelperLauncher{}
	svc.fallbackVibration = &launch_helper.Vibration{Pattern: []uint32{1}}
	c.Assert(svc.Start(), IsNil)

	// Persist is false so we just check the log
	card := &launch_helper.Card{Icon: "icon-value", Summary: "summary-value", Body: "body-value", Popup: true, Persist: false}
	vib := json.RawMessage(`true`)
	emb := &launch_helper.EmblemCounter{Count: 2, Visible: true}
	output := &launch_helper.HelperOutput{Notification: &launch_helper.Notification{Card: card, EmblemCounter: emb, RawVibration: vib}}
	b := svc.messageHandler(&click.AppId{}, "", output)
	c.Assert(b, Equals, true)
	args := testibus.GetCallArgs(endp)
	l := len(args)
	if l < 4 {
		c.Fatal("not enough elements in response from GetCallArgs")
	}
	mm := make([]string, 4)
	for i, m := range args[l-4:] {
		mm[i] = m.Member
	}
	sort.Strings(mm)
	// check the Present() methods were called.
	// For dbus-backed presenters, just check the right dbus methods are called
	c.Check(mm, DeepEquals, []string{"::SetProperty", "::SetProperty", "Notify", "VibratePattern"})
	// For the other ones, check the logs
	c.Check(ps.log.Captured(), Matches, `(?sm).* no persistable card:.*`)
	c.Check(ps.log.Captured(), Matches, `(?sm).* notification has no Sound:.*`)
}

func (ps *postalSuite) TestMessageHandlerReportsFailedNotifies(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), 1)
	nopTicker := make(chan []interface{})
	testibus.SetWatchSource(endp, "ActionInvoked", nopTicker)
	defer close(nopTicker)
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.NotificationsEndp = endp
	c.Assert(svc.Start(), IsNil)
	card := &launch_helper.Card{Icon: "icon-value", Summary: "summary-value", Body: "body-value", Popup: true}
	notif := &launch_helper.Notification{Card: card}
	output := &launch_helper.HelperOutput{Notification: notif}
	b := svc.messageHandler(&click.AppId{}, "", output)
	c.Check(b, Equals, false)
}

func (ps *postalSuite) TestMessageHandlerInhibition(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), []windowstack.WindowsInfo{{0, "com.example.test_test-app", true, 0}})
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.WindowStackEndp = endp
	c.Assert(svc.Start(), IsNil)
	output := &launch_helper.HelperOutput{Notification: &launch_helper.Notification{}} // Doesn't matter
	b := svc.messageHandler(clickhelp.MustParseAppId("com.example.test_test-app_0"), "", output)
	c.Check(b, Equals, false)
	c.Check(ps.log.Captured(), Matches, `(?sm).* notification skipped because app is focused.*`)
}

func (ps *postalSuite) TestMessageHandlerReportsButIgnoresUnmarshalErrors(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	c.Assert(svc.Start(), IsNil)
	output := &launch_helper.HelperOutput{[]byte(`broken`), nil}
	b := svc.messageHandler(nil, "", output)
	c.Check(b, Equals, false)
	c.Check(ps.log.Captured(), Matches, "(?msi).*skipping notification: nil.*")
}

func (ps *postalSuite) TestMessageHandlerReportsButIgnoresNilNotifies(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	c.Assert(svc.Start(), IsNil)
	svc.NotificationsEndp = endp
	output := &launch_helper.HelperOutput{[]byte(`{}`), nil}
	b := svc.messageHandler(nil, "", output)
	c.Assert(b, Equals, false)
	c.Check(ps.log.Captured(), Matches, "(?msi).*skipping notification: nil.*")
}

func (ps *postalSuite) TestMessageHandlerInvalidAction(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	c.Assert(svc.Start(), IsNil)
	fakeDisp := new(fakeUrlDispatcher)
	svc.urlDispatcher = fakeDisp
	fakeDisp.NextTestURLResult = false
	card := launch_helper.Card{Actions: []string{"notsupported://test-app"}}
	output := &launch_helper.HelperOutput{Notification: &launch_helper.Notification{Card: &card}}
	appId := clickhelp.MustParseAppId("com.example.test_test-app_0")
	b := svc.messageHandler(appId, "", output)
	c.Check(b, Equals, false)
	fakeDisp.Lock.Lock()
	defer fakeDisp.Lock.Unlock()
	c.Assert(len(fakeDisp.DispatchCalls), Equals, 0)
	c.Assert(len(fakeDisp.TestURLCalls), Equals, 1)
	c.Assert(fakeDisp.TestURLCalls[0][appId.DispatchPackage()], DeepEquals, []string{"notsupported://test-app"})
}

func (ps *postalSuite) TestHandleActionsDispatches(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	fmm := new(fakeMM)
	app, _ := click.ParseAppId("com.example.test_test-app")
	c.Assert(svc.Start(), IsNil)
	fakeDisp := new(fakeUrlDispatcher)
	fakeDisp.DispatchDone = make(chan bool)
	svc.urlDispatcher = fakeDisp
	fakeDisp.NextTestURLResult = true
	svc.messagingMenu = fmm
	aCh := make(chan *notifications.RawAction)
	rCh := make(chan *reply.MMActionReply)
	go func() {
		aCh <- nil // just in case?
		aCh <- &notifications.RawAction{App: app, Action: "potato://", Nid: "xyzzy"}
		close(aCh)
	}()
	go svc.handleActions(aCh, rCh)
	takeNextBool(fakeDisp.DispatchDone)
	fakeDisp.Lock.Lock()
	defer fakeDisp.Lock.Unlock()
	c.Assert(len(fakeDisp.DispatchCalls), Equals, 1)
	c.Assert(fakeDisp.DispatchCalls[0][0], Equals, "potato://")
	c.Assert(fakeDisp.DispatchCalls[0][1], Equals, app.DispatchPackage())
	c.Check(fmm.calls, DeepEquals, []string{"remove:xyzzy:true"})
}

func (ps *postalSuite) TestHandleMMUActionsDispatches(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	c.Assert(svc.Start(), IsNil)
	fakeDisp := new(fakeUrlDispatcher)
	svc.urlDispatcher = fakeDisp
	fakeDisp.DispatchDone = make(chan bool)
	fakeDisp.NextTestURLResult = true
	app, _ := click.ParseAppId("com.example.test_test-app")
	aCh := make(chan *notifications.RawAction)
	rCh := make(chan *reply.MMActionReply)
	go func() {
		rCh <- nil // just in case?
		rCh <- &reply.MMActionReply{App: app, Action: "potato://", Notification: "foo.bar"}
		close(rCh)
	}()
	go svc.handleActions(aCh, rCh)
	takeNextBool(fakeDisp.DispatchDone)
	fakeDisp.Lock.Lock()
	defer fakeDisp.Lock.Unlock()
	c.Assert(len(fakeDisp.DispatchCalls), Equals, 1)
	c.Assert(fakeDisp.DispatchCalls[0][0], Equals, "potato://")
	c.Assert(fakeDisp.DispatchCalls[0][1], Equals, app.DispatchPackage())
}

func (ps *postalSuite) TestValidateActions(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	c.Assert(svc.Start(), IsNil)
	card := launch_helper.Card{Actions: []string{"potato://test-app"}}
	notif := &launch_helper.Notification{Card: &card}
	fakeDisp := new(fakeUrlDispatcher)
	svc.urlDispatcher = fakeDisp
	fakeDisp.NextTestURLResult = true
	b := svc.validateActions(clickhelp.MustParseAppId("com.example.test_test-app_0"), notif)
	c.Check(b, Equals, true)
}

func (ps *postalSuite) TestValidateActionsNoActions(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	card := launch_helper.Card{}
	notif := &launch_helper.Notification{Card: &card}
	b := svc.validateActions(clickhelp.MustParseAppId("com.example.test_test-app_0"), notif)
	c.Check(b, Equals, true)
}

func (ps *postalSuite) TestValidateActionsNoCard(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	notif := &launch_helper.Notification{}
	b := svc.validateActions(clickhelp.MustParseAppId("com.example.test_test-app_0"), notif)
	c.Check(b, Equals, true)
}

type fakeMM struct {
	calls []string
}

func (*fakeMM) Present(*click.AppId, string, *launch_helper.Notification) bool { return false }
func (*fakeMM) GetCh() chan *reply.MMActionReply                               { return nil }
func (*fakeMM) StartCleanupLoop()                                              {}
func (fmm *fakeMM) RemoveNotification(s string, b bool) {
	fmm.calls = append(fmm.calls, fmt.Sprintf("remove:%s:%t", s, b))
}
func (fmm *fakeMM) Clear(*click.AppId, ...string) int {
	fmm.calls = append(fmm.calls, "clear")
	return 42
}
func (fmm *fakeMM) Tags(*click.AppId) []string {
	fmm.calls = append(fmm.calls, "tags")
	return []string{"hello"}
}

func (ps *postalSuite) TestListPersistent(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	fmm := new(fakeMM)
	svc.messagingMenu = fmm

	itags, err := svc.listPersistent(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Assert(err, IsNil)
	c.Assert(itags, HasLen, 1)
	c.Assert(itags[0], FitsTypeOf, []string(nil))
	tags := itags[0].([]string)
	c.Check(tags, DeepEquals, []string{"hello"})
	c.Check(fmm.calls, DeepEquals, []string{"tags"})
}

func (ps *postalSuite) TestListPersistentErrors(c *C) {
	for i, s := range []struct {
		args []interface{}
		errt error
	}{
		{nil, ErrBadArgCount},
		{[]interface{}{}, ErrBadArgCount},
		{[]interface{}{1}, ErrBadArgType},
		{[]interface{}{anAppId, 2}, ErrBadArgCount},
		{[]interface{}{"bar"}, click.ErrInvalidAppId},
	} {
		reg, err := new(PostalService).listPersistent(aPackageOnBus, s.args, nil)
		c.Check(reg, IsNil, Commentf("iteration #%d", i))
		c.Check(err, Equals, s.errt, Commentf("iteration #%d", i))
	}
}

func (ps *postalSuite) TestClearPersistent(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	fmm := new(fakeMM)
	svc.messagingMenu = fmm

	icleared, err := svc.clearPersistent(aPackageOnBus, []interface{}{anAppId, "one", ""}, nil)
	c.Assert(err, IsNil)
	c.Assert(icleared, HasLen, 1)
	c.Check(icleared[0], Equals, uint32(42))
}

func (ps *postalSuite) TestClearPersistentErrors(c *C) {
	for i, s := range []struct {
		args []interface{}
		err  error
	}{
		{[]interface{}{}, ErrBadArgCount},
		{[]interface{}{42}, ErrBadArgType},
		{[]interface{}{"xyzzy"}, click.ErrInvalidAppId},
		{[]interface{}{anAppId, 42}, ErrBadArgType},
		{[]interface{}{anAppId, "", 42}, ErrBadArgType},
	} {
		_, err := new(PostalService).clearPersistent(aPackageOnBus, s.args, nil)
		c.Check(err, Equals, s.err, Commentf("iter %d", i))
	}
}

func (ps *postalSuite) TestSetCounter(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	c.Check(svc.Start(), IsNil)

	_, err := svc.setCounter(aPackageOnBus, []interface{}{anAppId, int32(42), true}, nil)
	c.Assert(err, IsNil)

	quoted := "/" + string(nih.Quote([]byte(anAppId)))

	callArgs := testibus.GetCallArgs(svc.EmblemCounterEndp)
	c.Assert(callArgs, HasLen, 2)
	c.Check(callArgs[0].Member, Equals, "::SetProperty")
	c.Check(callArgs[0].Args, DeepEquals, []interface{}{"count", quoted, dbus.Variant{int32(42)}})

	c.Check(callArgs[1].Member, Equals, "::SetProperty")
	c.Check(callArgs[1].Args, DeepEquals, []interface{}{"countVisible", quoted, dbus.Variant{true}})
}

func (ps *postalSuite) TestSetCounterErrors(c *C) {
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.Start()

	for i, s := range []struct {
		args []interface{}
		err  error
	}{
		{[]interface{}{anAppId, int32(42), true}, nil}, // for reference
		{[]interface{}{}, ErrBadArgCount},
		{[]interface{}{anAppId}, ErrBadArgCount},
		{[]interface{}{anAppId, int32(42)}, ErrBadArgCount},
		{[]interface{}{anAppId, int32(42), true, "potato"}, ErrBadArgCount},
		{[]interface{}{"xyzzy", int32(42), true}, click.ErrInvalidAppId},
		{[]interface{}{1234567, int32(42), true}, ErrBadArgType},
		{[]interface{}{anAppId, "potatoe", true}, ErrBadArgType},
		{[]interface{}{anAppId, int32(42), "ru"}, ErrBadArgType},
	} {
		_, err := svc.setCounter(aPackageOnBus, s.args, nil)
		c.Check(err, Equals, s.err, Commentf("iter %d", i))
	}
}

func (ps *postalSuite) TestBlacklisted(c *C) {
	ps.winStackBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), []windowstack.WindowsInfo{},
		[]windowstack.WindowsInfo{},
		[]windowstack.WindowsInfo{},
		[]windowstack.WindowsInfo{})
	ps.unityGreeterBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), false, false, false, false)
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	svc.Start()
	ps.notifyEnabled = true

	emb := &launch_helper.EmblemCounter{Count: 2, Visible: true}
	card := &launch_helper.Card{Icon: "icon-value", Summary: "summary-value", Persist: true}
	output := &launch_helper.HelperOutput{Notification: &launch_helper.Notification{Card: card}}
	embOut := &launch_helper.HelperOutput{Notification: &launch_helper.Notification{EmblemCounter: emb}}
	app := clickhelp.MustParseAppId("com.example.app_app_1.0")
	// sanity check: things are presented as normal if notifyEnabled == true
	ps.notifyEnabled = true
	c.Check(svc.messageHandler(app, "0", output), Equals, true)
	c.Check(svc.messageHandler(app, "1", embOut), Equals, true)
	ps.notifyEnabled = false
	// and regular notifications (but not emblem counters) are suppressed if notifications are disabled.
	c.Check(svc.messageHandler(app, "2", output), Equals, false)
	c.Check(svc.messageHandler(app, "3", embOut), Equals, true)
}

func (ps *postalSuite) TestFocusedAppButLockedScreenNotification(c *C) {
	appId := "com.example.test_test-app"
	ps.winStackBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), []windowstack.WindowsInfo{{0, appId, true, 0}})
	ps.unityGreeterBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), true)
	svc := ps.replaceBuses(NewPostalService(ps.cfg, ps.log))
	// svc.WindowStackEndp = ps.winStackBus
	svc.Start()

	card := &launch_helper.Card{Icon: "icon-value", Summary: "summary-value", Persist: true}
	output := &launch_helper.HelperOutput{Notification: &launch_helper.Notification{Card: card}}
	app := clickhelp.MustParseAppId(fmt.Sprintf("%v_0", appId))

	c.Check(svc.messageHandler(app, "0", output), Equals, true)
}
