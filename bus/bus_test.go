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
	"io/ioutil"
	"launchpad.net/go-dbus/v1"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/logger"
	"os"
	"testing"
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type BusSuite struct{}

var nullog = logger.NewSimpleLogger(ioutil.Discard, "error")
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

// Tests that we can connect to the *actual* system bus.
// XXX maybe connect to a mock/fake/etc bus?
func (s *BusSuite) TestConnect(c *C) {
	b, err := SystemBus.Connect(Address{"", "", ""}, nullog)
	c.Assert(err, IsNil)
	defer b.Close()
}

// Test that if we try to connect to the session bus when no session
// bus is available, we get a reasonable result (i.e., an error).
func (s *BusSuite) TestConnectCanFail(c *C) {
	db := "DBUS_SESSION_BUS_ADDRESS"
	odb := os.Getenv(db)
	defer os.Setenv(db, odb)
	os.Setenv(db, "")

	_, err := SessionBus.Connect(Address{"", "", ""}, nullog)
	c.Check(err, NotNil)
}
