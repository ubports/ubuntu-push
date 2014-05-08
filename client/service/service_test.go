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
	"testing"

	. "launchpad.net/gocheck"

	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

func TestService(t *testing.T) { TestingT(t) }

type serviceSuite struct {
	log logger.Logger
}

var _ = Suite(&serviceSuite{})

func (ss *serviceSuite) SetUpSuite(c *C) {
	ss.log = helpers.NewTestLogger(c, "debug")
}

func (ss *serviceSuite) TestStart(c *C) {
	svc := &Service{Log: ss.log}
	c.Check(svc.IsStarted(), Equals, false)
	c.Check(svc.Start(), IsNil)
	c.Check(svc.IsStarted(), Equals, true)
}

func (ss *serviceSuite) TestStartTwice(c *C) {
	svc := &Service{Log: ss.log}
	c.Check(svc.Start(), IsNil)
	c.Check(svc.Start(), Equals, AlreadyStarted)
}

func (ss *serviceSuite) TestStartNoLog(c *C) {
	svc := &Service{}
	c.Check(svc.Start(), Equals, NotConfigured)
}

func (ss *serviceSuite) TestStartConnectsBus(c *C) {
	svc := &Service{Log: ss.log}
	c.Check(svc.Start(), IsNil)
	c.Check(svc.Bus, NotNil)
}

func (ss *serviceSuite) TestStartDoesNotOverwriteBus(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(true), nil)
	svc := &Service{Bus: bus, Log: ss.log}
	c.Check(svc.Start(), IsNil)
	c.Check(svc.Bus, Equals, bus)
}

func (ss *serviceSuite) TestStartFailsOnBusDialFailure(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(false), nil)
	svc := &Service{Bus: bus, Log: ss.log}
	c.Check(svc.Start(), ErrorMatches, `.*(?i)cond said no.*`)
}

func (ss *serviceSuite) TestStartGrabsName(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(true), nil)
	svc := &Service{Bus: bus, Log: ss.log}
	c.Assert(svc.Start(), IsNil)
}
