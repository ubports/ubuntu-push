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

package bus

import (
	"fmt"
	"launchpad.net/go-dbus"
	. "launchpad.net/gocheck"
	helpers "github.com/ubports/ubuntu-push/testing"
	"testing"
)

// hook up gocheck
func TestBus(t *testing.T) { TestingT(t) }

type BusSuite struct{}

var _ = Suite(&BusSuite{})

// Test we stringify sanely
func (s *BusSuite) TestBusType(c *C) {
	c.Check(fmt.Sprintf("%s", SystemBus), Equals, "SystemBus")
	c.Check(fmt.Sprintf("%s", SessionBus), Equals, "SessionBus")
}

// Test we get the right DBus bus
func (s *BusSuite) TestDBusType(c *C) {
	c.Check(SystemBus.(concreteBus).dbusType(), DeepEquals, dbus.SystemBus)
	c.Check(SessionBus.(concreteBus).dbusType(), DeepEquals, dbus.SessionBus)
}

// Tests that we can get an endpoint back
func (s *BusSuite) TestEndpoint(c *C) {
	endp := SystemBus.Endpoint(Address{"", "", ""}, helpers.NewTestLogger(c, "debug"))
	c.Assert(endp, NotNil)
}
