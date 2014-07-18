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
	"io/ioutil"
	"os"
	"sort"
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/notifications"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/bus/windowstack"
	"launchpad.net/ubuntu-push/click"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/launch_helper/cual"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/messaging/reply"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
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

func installTickMessageHandler(svc *PostalService) chan error {
	ch := make(chan error)
	msgHandler := svc.GetMessageHandler()
	svc.SetMessageHandler(func(app *click.AppId, nid string, output *launch_helper.HelperOutput) error {
		var err error
		if msgHandler != nil {
			err = msgHandler(app, nid, output)
		}
		ch <- err
		return err
	})
	return ch
}

type basePostalSuite struct {
	log         *helpers.TestLogger
	bus         bus.Endpoint
	notifBus    bus.Endpoint
	counterBus  bus.Endpoint
	hapticBus   bus.Endpoint
	urlDispBus  bus.Endpoint
	winStackBus bus.Endpoint
}

type postalSuite struct {
	basePostalSuite
}

var _ = Suite(&postalSuite{})

func (ss *postalSuite) SetUpSuite(c *C) {
	//	useTrivialHelper = true
}

func (bs *basePostalSuite) SetUpTest(c *C) {
	bs.log = helpers.NewTestLogger(c, "debug")
	bs.bus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true))
	bs.notifBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true))
	bs.counterBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true))
	bs.hapticBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true))
	bs.urlDispBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true))
	bs.winStackBus = testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), []windowstack.WindowsInfo{})
}

func (bs *basePostalSuite) replaceBuses(pst *PostalService) *PostalService {
	pst.Bus = bs.bus
	pst.NotificationsEndp = bs.notifBus
	pst.EmblemCounterEndp = bs.counterBus
	pst.HapticEndp = bs.hapticBus
	pst.URLDispatcherEndp = bs.urlDispBus
	pst.WindowStackEndp = bs.winStackBus
	return pst
}

func (ss *postalSuite) TestStart(c *C) {
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	c.Check(svc.IsRunning(), Equals, false)
	c.Check(svc.Start(), IsNil)
	c.Check(svc.IsRunning(), Equals, true)
	svc.Stop()
}

func (ss *postalSuite) TestStartTwice(c *C) {
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	c.Check(svc.Start(), IsNil)
	c.Check(svc.Start(), Equals, ErrAlreadyStarted)
	svc.Stop()
}

func (ss *postalSuite) TestStartNoLog(c *C) {
	svc := ss.replaceBuses(NewPostalService(nil, nil))
	c.Check(svc.Start(), Equals, ErrNotConfigured)
}

func (ss *postalSuite) TestStartNoBus(c *C) {
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	svc.Bus = nil
	c.Check(svc.Start(), Equals, ErrNotConfigured)

	svc = ss.replaceBuses(NewPostalService(nil, ss.log))
	svc.NotificationsEndp = nil
	c.Check(svc.Start(), Equals, ErrNotConfigured)
}

func (ss *postalSuite) TestTakeTheBusFail(c *C) {
	nEndp := testibus.NewMultiValuedTestingEndpoint(condition.Work(true), condition.Work(false))
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	svc.NotificationsEndp = nEndp
	_, err := svc.takeTheBus()
	c.Check(err, NotNil)
}

func (ss *postalSuite) TestTakeTheBusOk(c *C) {
	nEndp := testibus.NewMultiValuedTestingEndpoint(condition.Work(true), condition.Work(true), []interface{}{uint32(1), "hello"})
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	svc.NotificationsEndp = nEndp
	_, err := svc.takeTheBus()
	c.Check(err, IsNil)
}

func (ss *postalSuite) TestStartFailsOnBusDialFailure(c *C) {
	// XXX actually, we probably want to autoredial this
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	svc.Bus = testibus.NewTestingEndpoint(condition.Work(false), nil)
	c.Check(svc.Start(), ErrorMatches, `.*(?i)cond said no.*`)
	svc.Stop()
}

func (ss *postalSuite) TestStartGrabsName(c *C) {
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	c.Assert(svc.Start(), IsNil)
	callArgs := testibus.GetCallArgs(ss.bus)
	defer svc.Stop()
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[0].Member, Equals, "::GrabName")
}

func (ss *postalSuite) TestStopClosesBus(c *C) {
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	c.Assert(svc.Start(), IsNil)
	svc.Stop()
	callArgs := testibus.GetCallArgs(ss.bus)
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[len(callArgs)-1].Member, Equals, "::Close")
}

//
// Post() tests

func (is *integrationPostalSuite) TestPostWorks(c *C) {
	svc := is.replaceBuses(NewPostalService(nil, is.log))
	svc.msgHandler = nil
	ch := installTickMessageHandler(svc)
	c.Assert(svc.Start(), IsNil)
	rvs, err := svc.post(aPackageOnBus, []interface{}{anAppId, `{"message":{"world":1}}`}, nil)
	c.Assert(err, IsNil)
	c.Check(rvs, IsNil)
	rvs, err = svc.post(aPackageOnBus, []interface{}{anAppId, `{"message":{"moon":1}}`}, nil)
	c.Assert(err, IsNil)
	c.Check(rvs, IsNil)

	// wait for the two posts to "launch"
	takeNextBool(is.fakeInstance.ch)
	takeNextBool(is.fakeInstance.ch)

	x, ok := svc.HelperLauncher.(cual.UAL)
	c.Assert(ok, Equals, true)
	go x.OneDone("0")
	go x.OneDone("1")

	c.Check(takeNextError(ch), IsNil) // one,
	c.Check(takeNextError(ch), IsNil) // two posts
	c.Assert(svc.mbox, HasLen, 1)
	box, ok := svc.mbox[anAppId]
	c.Check(ok, Equals, true)
	msgs := box.AllMessages()
	c.Assert(msgs, HasLen, 2)
	c.Check(msgs[0], Equals, `{"world":1}`)
	c.Check(msgs[1], Equals, `{"moon":1}`)
}

func (ss *postalSuite) TestPostSignal(c *C) {
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	svc.msgHandler = nil

	hInp := &launch_helper.HelperInput{
		App: clickhelp.MustParseAppId(anAppId),
	}
	res := &launch_helper.HelperResult{Input: hInp}

	svc.handleHelperResult(res)

	// and check it fired the right signal
	callArgs := testibus.GetCallArgs(ss.bus)
	l := len(callArgs)
	if l < 1 {
		c.Fatal("not enough elements in resposne from GetCallArgs")
	}
	c.Check(callArgs[l-1].Member, Equals, "::Signal")
	c.Check(callArgs[l-1].Args, DeepEquals, []interface{}{"Post", aPackageOnBus, []interface{}{anAppId}})
}

func (ss *postalSuite) TestPostFailsIfPostFails(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(true),
		condition.Work(false))
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	svc.Bus = bus
	svc.SetMessageHandler(func(*click.AppId, string, *launch_helper.HelperOutput) error { return errors.New("fail") })
	_, err := svc.post("/hello", []interface{}{"xyzzy"}, nil)
	c.Check(err, NotNil)
}

func (ss *postalSuite) TestPostFailsIfBadArgs(c *C) {
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
		{[]interface{}{"bar", "hello"}, ErrBadAppId},
	} {
		reg, err := new(PostalService).post(aPackageOnBus, s.args, nil)
		c.Check(reg, IsNil, Commentf("iteration #%d", i))
		c.Check(err, Equals, s.errt, Commentf("iteration #%d", i))
	}
}

//
// Post (Broadcast) tests

type fakeHelperState struct {
	i  int
	ch chan bool
}

func (fhs *fakeHelperState) InstallObserver() error { return nil }
func (fhs *fakeHelperState) RemoveObserver() error  { return nil }
func (fhs *fakeHelperState) Stop(_, _ string) error { return nil }
func (fhs *fakeHelperState) Launch(_, _, f1, f2 string) (string, error) {
	dat, err := ioutil.ReadFile(f1)
	if err != nil {
		return "", err
	}
	err = ioutil.WriteFile(f2, dat, os.ModeTemporary)
	if err != nil {
		return "", err
	}

	id := []string{"0", "1", "2"}[fhs.i]
	fhs.i++

	fhs.ch <- true

	return id, nil
}

type integrationPostalSuite struct {
	basePostalSuite
	oldHelperState func(logger.Logger, cual.UAL) cual.HelperState
	oldHelperInfo  func(*click.AppId) (string, string)
	fakeInstance   *fakeHelperState
}

var _ = Suite(&integrationPostalSuite{})

func (ss *integrationPostalSuite) newFake(logger.Logger, cual.UAL) cual.HelperState {
	return ss.fakeInstance
}

func (is *integrationPostalSuite) SetUpSuite(c *C) {
	is.oldHelperState = launch_helper.NewHelperState
	is.oldHelperInfo = launch_helper.HelperInfo
	launch_helper.NewHelperState = is.newFake
	launch_helper.HelperInfo = func(*click.AppId) (string, string) { return "helpId", "bar" }

}

func (is *integrationPostalSuite) SetUpTest(c *C) {
	is.basePostalSuite.SetUpTest(c)
	is.fakeInstance = &fakeHelperState{ch: make(chan bool)}
}

func (is *integrationPostalSuite) TearDownSuite(c *C) {
	launch_helper.NewHelperState = is.oldHelperState
	launch_helper.HelperInfo = is.oldHelperInfo
}

func (is *integrationPostalSuite) TestPostBroadcast(c *C) {

	bus := testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), uint32(1))
	svc := is.replaceBuses(NewPostalService(nil, is.log))

	ch := installTickMessageHandler(svc)
	svc.NotificationsEndp = bus
	c.Assert(svc.Start(), IsNil)

	x, ok := svc.HelperLauncher.(cual.UAL)
	c.Assert(ok, Equals, true)
	err := svc.PostBroadcast()
	takeNextBool(is.fakeInstance.ch)
	go x.OneDone("0")

	c.Assert(err, IsNil)
	c.Check(takeNextError(ch), IsNil)
	// and check it fired the right signal (twice)
	callArgs := testibus.GetCallArgs(bus)
	c.Assert(callArgs, HasLen, 1)
	c.Check(callArgs[0].Member, Equals, "Notify")
	c.Check(callArgs[0].Args[0:6], DeepEquals, []interface{}{"_ubuntu-push-client", uint32(0), "update_manager_icon",
		"There's an updated system image.", "Tap to open the system updater.",
		[]string{`{"app":"_ubuntu-push-client","act":"Switch to app","nid":"settings:///system/system-update"}`, "Switch to app"}})
	// TODO: check the map in callArgs?
	// c.Check(callArgs[0].Args[7]["x-canonical-secondary-icon"], NotNil)
	// c.Check(callArgs[0].Args[7]["x-canonical-snap-decisions"], NotNil)
}

func (ss *postalSuite) TestPostBroadcastDoesNotFail(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(true),
		condition.Work(false))
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	c.Assert(svc.Start(), IsNil)
	svc.NotificationsEndp = bus
	svc.SetMessageHandler(func(*click.AppId, string, *launch_helper.HelperOutput) error {
		ss.log.Debugf("about to fail")
		return errors.New("fail")
	})
	ch := installTickMessageHandler(svc)
	err := svc.PostBroadcast()
	c.Check(takeNextError(ch), NotNil) // the messagehandler failed
	c.Check(err, IsNil)                // but broadcast was oblivious
	c.Check(ss.log.Captured(), Matches, `(?sm).*about to fail$`)
}

//
// Notifications tests
func (ss *postalSuite) TestNotificationsWorks(c *C) {
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
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

func (ss *postalSuite) TestNotificationsFailsIfBadArgs(c *C) {
	for i, s := range []struct {
		args []interface{}
		errt error
	}{
		{nil, ErrBadArgCount},
		{[]interface{}{}, ErrBadArgCount},
		{[]interface{}{1}, ErrBadArgType},
		{[]interface{}{"potato"}, ErrBadAppId},
	} {
		reg, err := new(PostalService).popAll(aPackageOnBus, s.args, nil)
		c.Check(reg, IsNil, Commentf("iteration #%d", i))
		c.Check(err, Equals, s.errt, Commentf("iteration #%d", i))
	}
}

func (ss *postalSuite) TestMessageHandlerPublicAPI(c *C) {
	svc := new(PostalService)
	c.Assert(svc.msgHandler, IsNil)
	var ext = &launch_helper.HelperOutput{}
	e := errors.New("Hello")
	f := func(_ *click.AppId, _ string, s *launch_helper.HelperOutput) error { ext = s; return e }
	c.Check(svc.GetMessageHandler(), IsNil)
	svc.SetMessageHandler(f)
	c.Check(svc.GetMessageHandler(), NotNil)
	hOutput := &launch_helper.HelperOutput{[]byte("37"), nil}
	c.Check(svc.msgHandler(nil, "", hOutput), Equals, e)
	c.Check(ext, DeepEquals, hOutput)
}

func (ss *postalSuite) TestPostCallsMessageHandler(c *C) {
	ch := make(chan *launch_helper.HelperOutput)
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	c.Assert(svc.Start(), IsNil)
	// check the message handler gets called
	f := func(_ *click.AppId, _ string, s *launch_helper.HelperOutput) error { ch <- s; return nil }
	svc.SetMessageHandler(f)
	c.Check(svc.Post(&click.AppId{}, "thing", json.RawMessage("{}")), IsNil)
	c.Check(takeNextHelperOutput(ch), DeepEquals, &launch_helper.HelperOutput{Message: []byte("{}")})
	err := errors.New("ouch")
	svc.SetMessageHandler(func(*click.AppId, string, *launch_helper.HelperOutput) error { return err })
	// but the error doesn't bubble out
	c.Check(svc.Post(&click.AppId{}, "", json.RawMessage("{}")), IsNil)
}

func (ss *postalSuite) TestMessageHandlerPresents(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), uint32(1))
	svc := NewPostalService(nil, ss.log)
	svc.Bus = endp
	svc.EmblemCounterEndp = endp
	svc.HapticEndp = endp
	svc.NotificationsEndp = endp
	svc.URLDispatcherEndp = ss.urlDispBus
	svc.WindowStackEndp = ss.winStackBus
	c.Assert(svc.Start(), IsNil)

	// Persist is false so we just check the log
	card := &launch_helper.Card{Icon: "icon-value", Summary: "summary-value", Body: "body-value", Popup: true, Persist: false}
	vib := &launch_helper.Vibration{Duration: 500}
	emb := &launch_helper.EmblemCounter{Count: 2, Visible: true}
	output := &launch_helper.HelperOutput{Notification: &launch_helper.Notification{Card: card, EmblemCounter: emb, Vibrate: vib}}
	err := svc.messageHandler(&click.AppId{}, "", output)
	c.Assert(err, IsNil)
	args := testibus.GetCallArgs(endp)
	l := len(args)
	if l < 4 {
		c.Fatal("not enough elements in resposne from GetCallArgs")
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
	c.Check(ss.log.Captured(), Matches, `(?sm).* no persistable card:.*`)
	c.Check(ss.log.Captured(), Matches, `(?sm).* no Sound in the notification.*`)
}

func (ss *postalSuite) TestMessageHandlerReportsFailedNotifies(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), 1)
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	svc.NotificationsEndp = endp
	c.Assert(svc.Start(), IsNil)
	card := &launch_helper.Card{Icon: "icon-value", Summary: "summary-value", Body: "body-value", Popup: true}
	notif := &launch_helper.Notification{Card: card}
	output := &launch_helper.HelperOutput{Notification: notif}
	err := svc.messageHandler(&click.AppId{}, "", output)
	c.Assert(err, NotNil)
}

func (ss *postalSuite) TestMessageHandlerInhibition(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Work(true), condition.Work(true), []windowstack.WindowsInfo{{0, "com.example.test_test-app", true, 0}})
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	svc.WindowStackEndp = endp
	c.Assert(svc.Start(), IsNil)
	output := &launch_helper.HelperOutput{} // Doesn't matter
	err := svc.messageHandler(clickhelp.MustParseAppId("com.example.test_test-app_0"), "", output)
	c.Check(err, IsNil)
	c.Check(ss.log.Captured(), Matches, `(?sm).* Notification skipped because app is focused.*`)
}

func (ss *postalSuite) TestMessageHandlerReportsButIgnoresUnmarshalErrors(c *C) {
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	c.Assert(svc.Start(), IsNil)
	output := &launch_helper.HelperOutput{[]byte(`broken`), nil}
	err := svc.messageHandler(nil, "", output)
	c.Check(err, IsNil)
	c.Check(ss.log.Captured(), Matches, "(?msi).*skipping notification: nil.*")
}

func (ss *postalSuite) TestMessageHandlerReportsButIgnoresNilNotifies(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Work(true), condition.Work(false))
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	c.Assert(svc.Start(), IsNil)
	svc.NotificationsEndp = endp
	output := &launch_helper.HelperOutput{[]byte(`{}`), nil}
	err := svc.messageHandler(nil, "", output)
	c.Assert(err, IsNil)
	c.Check(ss.log.Captured(), Matches, "(?msi).*skipping notification: nil.*")
}

func (ss *postalSuite) TestHandleActionsDispatches(c *C) {
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	c.Assert(svc.Start(), IsNil)
	aCh := make(chan *notifications.RawAction)
	rCh := make(chan *reply.MMActionReply)
	bCh := make(chan bool)
	go func() {
		aCh <- nil // just in case?
		aCh <- &notifications.RawAction{Action: "potato://"}
		close(aCh)
		bCh <- true
	}()
	go svc.handleActions(aCh, rCh)
	takeNextBool(bCh)
	args := testibus.GetCallArgs(ss.urlDispBus)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "DispatchURL")
	c.Assert(args[0].Args, HasLen, 1)
	c.Assert(args[0].Args[0], Equals, "potato://")
}

func (ss *postalSuite) TestHandleMMUActionsDispatches(c *C) {
	svc := ss.replaceBuses(NewPostalService(nil, ss.log))
	c.Assert(svc.Start(), IsNil)
	aCh := make(chan *notifications.RawAction)
	rCh := make(chan *reply.MMActionReply)
	bCh := make(chan bool)
	go func() {
		rCh <- nil // just in case?
		rCh <- &reply.MMActionReply{Action: "potato://", Notification: "foo.bar"}
		close(rCh)
		bCh <- true
	}()
	go svc.handleActions(aCh, rCh)
	takeNextBool(bCh)
	args := testibus.GetCallArgs(ss.urlDispBus)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "DispatchURL")
	c.Assert(args[0].Args, HasLen, 1)
	c.Assert(args[0].Args[0], Equals, "potato://")
}
