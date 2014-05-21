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
	"os"
	"testing"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/bus"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

func TestService(t *testing.T) { TestingT(t) }

type serviceSuite struct {
	log logger.Logger
	bus bus.Endpoint
}

var _ = Suite(&serviceSuite{})

func (ss *serviceSuite) SetUpTest(c *C) {
	ss.log = helpers.NewTestLogger(c, "debug")
	ss.bus = testibus.NewTestingEndpoint(condition.Work(true), nil)
}

func (ss *serviceSuite) TestStart(c *C) {
	svc := NewService(ss.bus, ss.log)
	c.Check(svc.IsRunning(), Equals, false)
	c.Check(svc.Start(), IsNil)
	c.Check(svc.IsRunning(), Equals, true)
	svc.Stop()
}

func (ss *serviceSuite) TestStartTwice(c *C) {
	svc := NewService(ss.bus, ss.log)
	c.Check(svc.Start(), IsNil)
	c.Check(svc.Start(), Equals, AlreadyStarted)
	svc.Stop()
}

func (ss *serviceSuite) TestStartNoLog(c *C) {
	svc := NewService(ss.bus, nil)
	c.Check(svc.Start(), Equals, NotConfigured)
}

func (ss *serviceSuite) TestStartNoBus(c *C) {
	svc := NewService(nil, ss.log)
	c.Check(svc.Start(), Equals, NotConfigured)
}

func (ss *serviceSuite) TestStartFailsOnBusDialFailure(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(false), nil)
	svc := NewService(bus, ss.log)
	c.Check(svc.Start(), ErrorMatches, `.*(?i)cond said no.*`)
	svc.Stop()
}

func (ss *serviceSuite) TestStartGrabsName(c *C) {
	svc := NewService(ss.bus, ss.log)
	c.Assert(svc.Start(), IsNil)
	callArgs := testibus.GetCallArgs(ss.bus)
	defer svc.Stop()
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[0].Member, Equals, "::GrabName")
}

func (ss *serviceSuite) TestStopClosesBus(c *C) {
	svc := NewService(ss.bus, ss.log)
	c.Assert(svc.Start(), IsNil)
	svc.Stop()
	callArgs := testibus.GetCallArgs(ss.bus)
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[len(callArgs)-1].Member, Equals, "::Close")
}

// registration tests

func (ss *serviceSuite) TestRegistrationFailsIfBadArgs(c *C) {
	for i, s := range []struct {
		args []interface{}
		errt error
	}{
		{nil, BadArgCount},                 // no args
		{[]interface{}{}, BadArgCount},     // still no args
		{[]interface{}{42}, BadArgType},    // bad arg type
		{[]interface{}{1, 2}, BadArgCount}, // too many args
	} {
		reg, err := new(Service).register(s.args, nil)
		c.Check(reg, IsNil, Commentf("iteration #%d", i))
		c.Check(err, Equals, s.errt, Commentf("iteration #%d", i))
	}
}

func (ss *serviceSuite) TestRegistrationWorks(c *C) {
	reg, err := new(Service).register([]interface{}{"this"}, nil)
	c.Assert(reg, HasLen, 1)
	regs, ok := reg[0].(string)
	c.Check(ok, Equals, true)
	c.Check(regs, Not(Equals), "")
	c.Check(err, IsNil)
}

func (ss *serviceSuite) TestRegistrationOverrideWorks(c *C) {
	os.Setenv("PUSH_REG_stuff", "42")
	defer os.Setenv("PUSH_REG_stuff", "")

	reg, err := new(Service).register([]interface{}{"stuff"}, nil)
	c.Assert(reg, HasLen, 1)
	regs, ok := reg[0].(string)
	c.Check(ok, Equals, true)
	c.Check(regs, Equals, "42")
	c.Check(err, IsNil)
}

//
// Injection tests

func (ss *serviceSuite) TestInjectWorks(c *C) {
	svc := NewService(ss.bus, ss.log)
	rvs, err := svc.inject([]interface{}{"hello", "world"}, nil)
	c.Assert(err, IsNil)
	c.Check(rvs, IsNil)
	rvs, err = svc.inject([]interface{}{"hello", "there"}, nil)
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
	c.Check(callArgs[0].Args, DeepEquals, []interface{}{"Notification", []interface{}{"hello"}})
	c.Check(callArgs[1], DeepEquals, callArgs[0])
}

func (ss *serviceSuite) TestInjectFailsIfInjectFails(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(true),
		condition.Work(false))
	svc := NewService(bus, ss.log)
	svc.SetMessageHandler(func([]byte) error { return errors.New("fail") })
	_, err := svc.inject([]interface{}{"hello", "xyzzy"}, nil)
	c.Check(err, NotNil)
}

func (ss *serviceSuite) TestInjectFailsIfBadArgs(c *C) {
	for i, s := range []struct {
		args []interface{}
		errt error
	}{
		{nil, BadArgCount},
		{[]interface{}{}, BadArgCount},
		{[]interface{}{1}, BadArgCount},
		{[]interface{}{1, 2}, BadArgType},
		{[]interface{}{"1", 2}, BadArgType},
		{[]interface{}{1, "2"}, BadArgType},
		{[]interface{}{1, 2, 3}, BadArgCount},
	} {
		reg, err := new(Service).inject(s.args, nil)
		c.Check(reg, IsNil, Commentf("iteration #%d", i))
		c.Check(err, Equals, s.errt, Commentf("iteration #%d", i))
	}
}

//
// Notifications tests
func (ss *serviceSuite) TestNotificationsWorks(c *C) {
	svc := NewService(ss.bus, ss.log)
	nots, err := svc.notifications([]interface{}{"hello"}, nil)
	c.Assert(err, IsNil)
	c.Assert(nots, NotNil)
	c.Assert(nots, HasLen, 1)
	c.Check(nots[0], HasLen, 0)
	if svc.mbox == nil {
		svc.mbox = make(map[string][]string)
	}
	svc.mbox["hello"] = append(svc.mbox["hello"], "this", "thing")
	nots, err = svc.notifications([]interface{}{"hello"}, nil)
	c.Assert(err, IsNil)
	c.Assert(nots, NotNil)
	c.Assert(nots, HasLen, 1)
	c.Check(nots[0], DeepEquals, []string{"this", "thing"})
}

func (ss *serviceSuite) TestNotificationsFailsIfBadArgs(c *C) {
	for i, s := range []struct {
		args []interface{}
		errt error
	}{
		{nil, BadArgCount},                 // no args
		{[]interface{}{}, BadArgCount},     // still no args
		{[]interface{}{42}, BadArgType},    // bad arg type
		{[]interface{}{1, 2}, BadArgCount}, // too many args
	} {
		reg, err := new(Service).notifications(s.args, nil)
		c.Check(reg, IsNil, Commentf("iteration #%d", i))
		c.Check(err, Equals, s.errt, Commentf("iteration #%d", i))
	}
}

func (ss *serviceSuite) TestMessageHandler(c *C) {
	svc := new(Service)
	c.Assert(svc.msgHandler, IsNil)
	var ext = []byte{}
	e := errors.New("Hello")
	f := func(s []byte) error { ext = s; return e }
	c.Check(svc.GetMessageHandler(), IsNil)
	svc.SetMessageHandler(f)
	c.Check(svc.GetMessageHandler(), NotNil)
	c.Check(svc.msgHandler([]byte("37")), Equals, e)
	c.Check(ext, DeepEquals, []byte("37"))
}

func (ss *serviceSuite) TestInjectCallsMessageHandler(c *C) {
	var ext = []byte{}
	svc := NewService(ss.bus, ss.log)
	f := func(s []byte) error { ext = s; return nil }
	svc.SetMessageHandler(f)
	c.Check(svc.Inject("stuff", []byte("{}")), IsNil)
	c.Check(ext, DeepEquals, []byte("{}"))
	err := errors.New("ouch")
	svc.SetMessageHandler(func([]byte) error { return err })
	c.Check(svc.Inject("stuff", []byte("{}")), Equals, err)
}
