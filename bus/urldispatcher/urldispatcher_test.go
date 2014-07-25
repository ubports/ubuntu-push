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

package urldispatcher

import (
	. "launchpad.net/gocheck"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
	"testing"
)

// hook up gocheck
func TestUrldispatcher(t *testing.T) { TestingT(t) }

type UDSuite struct {
	log logger.Logger
}

var _ = Suite(&UDSuite{})

func (s *UDSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
}

func (s *UDSuite) TestDispatchURLWorks(c *C) {
	endp := testibus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{})
	ud := New(endp, s.log)
	appId := clickhelp.MustParseAppId("com.example.test_app_0.99")
	err := ud.DispatchURL("this", appId)
	c.Check(err, IsNil)
}

func (s *UDSuite) TestDispatchURLFailsIfCallFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	ud := New(endp, s.log)
	appId := clickhelp.MustParseAppId("com.example.test_app_0.99")
	err := ud.DispatchURL("this", appId)
	c.Check(err, NotNil)
}

func (s *UDSuite) TestTestURLWorks(c *C) {
	endp := testibus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{[]string{"com.example.test_app_0.99"}})
	ud := New(endp, s.log)
	appId := clickhelp.MustParseAppId("com.example.test_app_0.99")
	c.Check(ud.TestURL(appId, []string{"this"}), Equals, true)
}

func (s *UDSuite) TestTestURLFailsIfCallFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	ud := New(endp, s.log)
	appId := clickhelp.MustParseAppId("com.example.test_app_0.99")
	c.Check(ud.TestURL(appId, []string{"this"}), Equals, false)
}
