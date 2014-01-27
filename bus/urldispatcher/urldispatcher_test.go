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
	endp := testibus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{})
	ud := New(endp, nullog)
	err := ud.DispatchURL("this")
	c.Check(err, IsNil)
}

func (s *UDSuite) TestFailsIfCallFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	ud := New(endp, nullog)
	err := ud.DispatchURL("this")
	c.Check(err, NotNil)
}
