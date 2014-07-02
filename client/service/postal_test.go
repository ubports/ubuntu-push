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
	"errors"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/bus"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/launch_helper"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

type postalSuite struct {
	log      *helpers.TestLogger
	bus      bus.Endpoint
	notifBus bus.Endpoint
}

var _ = Suite(&postalSuite{})

func (ss *postalSuite) SetUpTest(c *C) {
	ss.log = helpers.NewTestLogger(c, "debug")
	ss.bus = testibus.NewTestingEndpoint(condition.Work(true), nil)
	ss.notifBus = testibus.NewTestingEndpoint(condition.Work(true), nil)
}

func (ss *postalSuite) TestStart(c *C) {
	svc := NewPostalService(ss.bus, ss.notifBus, ss.log)
	c.Check(svc.IsRunning(), Equals, false)
	c.Check(svc.Start(), IsNil)
	c.Check(svc.IsRunning(), Equals, true)
	svc.Stop()
}

func (ss *postalSuite) TestStartTwice(c *C) {
	svc := NewPostalService(ss.bus, ss.notifBus, ss.log)
	c.Check(svc.Start(), IsNil)
	c.Check(svc.Start(), Equals, AlreadyStarted)
	svc.Stop()
}

func (ss *postalSuite) TestStartNoLog(c *C) {
	svc := NewPostalService(ss.bus, ss.notifBus, nil)
	c.Check(svc.Start(), Equals, NotConfigured)
}

func (ss *postalSuite) TestStartNoBus(c *C) {
	svc := NewPostalService(nil, ss.notifBus, ss.log)
	c.Check(svc.Start(), Equals, NotConfigured)
}

func (ss *postalSuite) TestTakeTheBustFail(c *C) {
	nEndp := testibus.NewMultiValuedTestingEndpoint(condition.Work(true), condition.Work(false), []interface{}{uint32(1), "hello"})
	svc := NewPostalService(ss.bus, nEndp, ss.log)
	_, err := svc.TakeTheBus()
	c.Check(err, NotNil)
}

func (ss *postalSuite) TestTakeTheBustOk(c *C) {
	nEndp := testibus.NewMultiValuedTestingEndpoint(condition.Work(true), condition.Work(true), []interface{}{uint32(1), "hello"})
	svc := NewPostalService(ss.bus, nEndp, ss.log)
	_, err := svc.TakeTheBus()
	c.Check(err, IsNil)
}

func (ss *postalSuite) TestStartFailsOnBusDialFailure(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(false), nil)
	svc := NewPostalService(bus, ss.notifBus, ss.log)
	c.Check(svc.Start(), ErrorMatches, `.*(?i)cond said no.*`)
	svc.Stop()
}

func (ss *postalSuite) TestStartGrabsName(c *C) {
	svc := NewPostalService(ss.bus, ss.notifBus, ss.log)
	c.Assert(svc.Start(), IsNil)
	callArgs := testibus.GetCallArgs(ss.bus)
	defer svc.Stop()
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[0].Member, Equals, "::GrabName")
}

func (ss *postalSuite) TestStopClosesBus(c *C) {
	svc := NewPostalService(ss.bus, ss.notifBus, ss.log)
	c.Assert(svc.Start(), IsNil)
	svc.Stop()
	callArgs := testibus.GetCallArgs(ss.bus)
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[len(callArgs)-1].Member, Equals, "::Close")
}

//
// Injection tests

func (ss *postalSuite) TestInjectWorks(c *C) {
	svc := NewPostalService(ss.bus, ss.notifBus, ss.log)
	svc.msgHandler = nil
	rvs, err := svc.inject("/hello", []interface{}{"world"}, nil)
	c.Assert(err, IsNil)
	c.Check(rvs, IsNil)
	rvs, err = svc.inject("/hello", []interface{}{"there"}, nil)
	c.Assert(err, IsNil)
	c.Check(rvs, IsNil)
	c.Assert(svc.mbox, HasLen, 1)
	c.Assert(svc.mbox["hello"], HasLen, 2)
	c.Check(svc.mbox["hello"][0], Equals, "world")
	c.Check(svc.mbox["hello"][1], Equals, "there")

	// and check it fired the right signal (twice)
	callArgs := testibus.GetCallArgs(ss.bus)
	c.Assert(callArgs, HasLen, 2)
	c.Check(callArgs[0].Member, Equals, "::Signal")
	c.Check(callArgs[0].Args, DeepEquals, []interface{}{"Notification", "/hello", []interface{}{"hello"}})
	c.Check(callArgs[1], DeepEquals, callArgs[0])
}

func (ss *postalSuite) TestInjectFailsIfInjectFails(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(true),
		condition.Work(false))
	svc := NewPostalService(bus, ss.notifBus, ss.log)
	svc.SetMessageHandler(func(string, string, *launch_helper.HelperOutput) error { return errors.New("fail") })
	_, err := svc.inject("/hello", []interface{}{"xyzzy"}, nil)
	c.Check(err, NotNil)
}

func (ss *postalSuite) TestInjectFailsIfBadArgs(c *C) {
	for i, s := range []struct {
		args []interface{}
		errt error
	}{
		{nil, BadArgCount},
		{[]interface{}{}, BadArgCount},
		{[]interface{}{1}, BadArgType},
		{[]interface{}{1, 2}, BadArgCount},
	} {
		reg, err := new(PostalService).inject("", s.args, nil)
		c.Check(reg, IsNil, Commentf("iteration #%d", i))
		c.Check(err, Equals, s.errt, Commentf("iteration #%d", i))
	}
}

//
// Injection (Broadcast) tests

func (ss *postalSuite) TestInjectBroadcast(c *C) {
	bus := testibus.NewTestingEndpoint(nil, condition.Work(true), uint32(1))
	svc := NewPostalService(ss.bus, bus, ss.log)
	//svc.msgHandler = nil
	rvs, err := svc.InjectBroadcast()
	c.Assert(err, IsNil)
	c.Check(rvs, Equals, uint32(0))
	c.Assert(err, IsNil)
	// and check it fired the right signal (twice)
	callArgs := testibus.GetCallArgs(bus)
	c.Assert(callArgs, HasLen, 1)
	c.Check(callArgs[0].Member, Equals, "Notify")
	c.Check(callArgs[0].Args[0:6], DeepEquals, []interface{}{"ubuntu-push-client", uint32(0), "update_manager_icon",
		"There's an updated system image.", "Tap to open the system updater.",
		[]string{"ubuntu-push-client::settings:///system/system-update::0", "Switch to app"}})
	// TODO: check the map in callArgs?
	// c.Check(callArgs[0].Args[7]["x-canonical-secondary-icon"], NotNil)
	// c.Check(callArgs[0].Args[7]["x-canonical-snap-decisions"], NotNil)
}

func (ss *postalSuite) TestInjectBroadcastFails(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(true),
		condition.Work(false))
	svc := NewPostalService(ss.bus, bus, ss.log)
	svc.SetMessageHandler(func(string, string, *launch_helper.HelperOutput) error { return errors.New("fail") })
	_, err := svc.InjectBroadcast()
	c.Check(err, NotNil)
}

//
// Notifications tests
func (ss *postalSuite) TestNotificationsWorks(c *C) {
	svc := NewPostalService(ss.bus, ss.notifBus, ss.log)
	nots, err := svc.notifications("/hello", nil, nil)
	c.Assert(err, IsNil)
	c.Assert(nots, NotNil)
	c.Assert(nots, HasLen, 1)
	c.Check(nots[0], HasLen, 0)
	if svc.mbox == nil {
		svc.mbox = make(map[string][]string)
	}
	svc.mbox["hello"] = append(svc.mbox["hello"], "this", "thing")
	nots, err = svc.notifications("/hello", nil, nil)
	c.Assert(err, IsNil)
	c.Assert(nots, NotNil)
	c.Assert(nots, HasLen, 1)
	c.Check(nots[0], DeepEquals, []string{"this", "thing"})
}

func (ss *postalSuite) TestNotificationsFailsIfBadArgs(c *C) {
	reg, err := new(PostalService).notifications("/foo", []interface{}{1}, nil)
	c.Check(reg, IsNil)
	c.Check(err, Equals, BadArgCount)
}

func (ss *postalSuite) TestMessageHandlerPublicAPI(c *C) {
	svc := new(PostalService)
	c.Assert(svc.msgHandler, IsNil)
	var ext = &launch_helper.HelperOutput{}
	e := errors.New("Hello")
	f := func(app string, nid string, s *launch_helper.HelperOutput) error { ext = s; return e }
	c.Check(svc.GetMessageHandler(), IsNil)
	svc.SetMessageHandler(f)
	c.Check(svc.GetMessageHandler(), NotNil)
	hOutput := &launch_helper.HelperOutput{[]byte("37"), nil}
	c.Check(svc.msgHandler("", "", hOutput), Equals, e)
	c.Check(ext, DeepEquals, hOutput)
}

func (ss *postalSuite) TestInjectCallsMessageHandler(c *C) {
	var ext = &launch_helper.HelperOutput{}
	svc := NewPostalService(ss.bus, ss.notifBus, ss.log)
	f := func(app string, nid string, s *launch_helper.HelperOutput) error { ext = s; return nil }
	svc.SetMessageHandler(f)
	c.Check(svc.Inject("stuff", "thing", "{}"), IsNil)
	c.Check(ext, DeepEquals, &launch_helper.HelperOutput{})
	err := errors.New("ouch")
	svc.SetMessageHandler(func(string, string, *launch_helper.HelperOutput) error { return err })
	c.Check(svc.Inject("stuff", "", "{}"), Equals, err)
}

func (ss *postalSuite) TestMessageHandler(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true), uint32(1))
	svc := NewPostalService(ss.bus, endp, ss.log)
	card := &launch_helper.Card{Icon: "icon-value", Summary: "summary-value", Body: "body-value", Popup: true}
	output := &launch_helper.HelperOutput{Notification: &launch_helper.Notification{Card: card}}
	err := svc.messageHandler("xyzzy", "", output)
	c.Assert(err, IsNil)
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "Notify")
	c.Check(args[0].Args[0], Equals, "xyzzy")
	c.Check(args[0].Args[2], Equals, "icon-value")
	c.Check(args[0].Args[3], Equals, "summary-value")
	c.Check(args[0].Args[4], Equals, "body-value")
}

func (ss *postalSuite) TestMessageHandlerReportsFailedNotifies(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	svc := NewPostalService(ss.bus, endp, ss.log)
	card := &launch_helper.Card{Icon: "icon-value", Summary: "summary-value", Body: "body-value", Popup: true}
	notif := &launch_helper.Notification{Card: card}
	output := &launch_helper.HelperOutput{Notification: notif}
	err := svc.messageHandler("", "", output)
	c.Assert(err, NotNil)
}

func (ss *postalSuite) TestMessageHandlerReportsButIgnoresUnmarshalErrors(c *C) {
	svc := NewPostalService(ss.bus, ss.notifBus, ss.log)
	output := &launch_helper.HelperOutput{[]byte(`broken`), nil}
	err := svc.messageHandler("", "", output)
	c.Check(err, IsNil)
	c.Check(ss.log.Captured(), Matches, "(?msi).*skipping notification: nil.*")
}

func (ss *postalSuite) TestMessageHandlerReportsButIgnoresNilNotifies(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	svc := NewPostalService(ss.bus, endp, ss.log)
	output := &launch_helper.HelperOutput{[]byte(`{}`), nil}
	err := svc.messageHandler("", "", output)
	c.Assert(err, IsNil)
	c.Check(ss.log.Captured(), Matches, "(?msi).*skipping notification: nil.*")
}
