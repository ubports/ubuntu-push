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

package testing

import (
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/testing/condition"
	"testing"
)

// hook up gocheck
func BusTest(t *testing.T) { TestingT(t) }

type TestingBusSuite struct{}

var _ = Suite(&TestingBusSuite{})

// Test Connect() on a working bus returns an endpoint that looks right
func (s *TestingBusSuite) TestConnectWorks(c *C) {
	addr := bus.Address{"", "", ""}
	tb := NewTestingBus(condition.Work(true), condition.Work(false), 42, 42, 42)
	endp, err := tb.Connect(addr, nil)
	c.Check(err, IsNil)
	c.Check(endp, FitsTypeOf, &testingEndpoint{})
	c.Check(endp.(*testingEndpoint).cond.OK(), Equals, false)
	c.Check(endp.(*testingEndpoint).retvals, HasLen, 3)
}

// Test Connect() on a working "multi-valued" bus returns an endpoint that looks right
func (s *TestingBusSuite) TestConnectMultiValued(c *C) {
	addr := bus.Address{"", "", ""}
	tb := NewMultiValuedTestingBus(condition.Work(true), condition.Work(true),
		[]interface{}{42, 17},
		[]interface{}{42, 17, 13},
		[]interface{}{42},
	)
	endpp, err := tb.Connect(addr, nil)
	c.Check(err, IsNil)
	endp, ok := endpp.(*testingEndpoint)
	c.Check(ok, Equals, true)
	c.Check(endp.cond.OK(), Equals, true)
	c.Assert(endp.retvals, HasLen, 3)
	c.Check(endp.retvals[0], HasLen, 2)
	c.Check(endp.retvals[1], HasLen, 3)
	c.Check(endp.retvals[2], HasLen, 1)
}

// Test Connect() with a non-working bus fails
func (s *TestingBusSuite) TestConnectNoWork(c *C) {
	addr := bus.Address{"", "", ""}
	tb := NewTestingBus(condition.Work(false), condition.Work(true))
	_, err := tb.Connect(addr, nil)
	c.Check(err, NotNil)
}

// Test TestingBus stringifies sanely
func (s *TestingBusSuite) TestStringifyBus(c *C) {
	tb := NewTestingBus(condition.Work(false), condition.Work(true))
	c.Check(tb.String(), Matches, ".*TestingBus.*")
}
