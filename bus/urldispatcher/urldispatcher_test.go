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
	"io/ioutil"
	. "launchpad.net/gocheck"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/testing/condition"
	"testing"
)

// hook up gocheck
func TestUrldispatcher(t *testing.T) { TestingT(t) }

type UDSuite struct{}

var _ = Suite(&UDSuite{})

var nullog = logger.NewSimpleLogger(ioutil.Discard, "error")

func (s *UDSuite) TestWorks(c *C) {
	tb := testibus.NewMultiValuedTestingBus(condition.Work(true), condition.Work(true), []interface{}{})
	ud, err := New(tb, nullog)
	c.Assert(err, IsNil)
	err = ud.DispatchURL("this")
	c.Check(err, IsNil)
}

func (s *UDSuite) TestFailsIfConnectFails(c *C) {
	tb := testibus.NewTestingBus(condition.Work(false), condition.Work(true))
	_, err := New(tb, nullog)
	c.Check(err, NotNil)
}

func (s *UDSuite) TestFailsIfCallFails(c *C) {
	tb := testibus.NewTestingBus(condition.Work(true), condition.Work(false))
	ud, err := New(tb, nullog)
	c.Assert(err, IsNil)
	err = ud.DispatchURL("this")
	c.Check(err, NotNil)
}
