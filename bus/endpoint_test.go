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
	. "launchpad.net/gocheck"
	"os"
	"testing"
)

// hook up gocheck
func EndpointTest(t *testing.T) { TestingT(t) }

type EndpointSuite struct{}

var _ = Suite(&EndpointSuite{})

// TODO: this is going to remain empty until go-dbus grows some
// testing amenities (already talked about it with jamesh)

// Tests that we can connect to the *actual* system bus.
// XXX maybe connect to a mock/fake/etc bus?
func (s *BusSuite) TestDial(c *C) {
	endp := newEndpoint(SystemBus, Address{"", "", ""}, nullog)
	c.Assert(endp.bus, IsNil)
	err := endp.Dial()
	c.Check(err, IsNil)
	c.Assert(endp.bus, NotNil)
	endp.Close()
	c.Assert(endp.bus, IsNil) // Close cleans up
}

// Test that if we try to connect to the session bus when no session
// bus is available, we get a reasonable result (i.e., an error).
func (s *BusSuite) TestDialCanFail(c *C) {
	db := "DBUS_SESSION_BUS_ADDRESS"
	odb := os.Getenv(db)
	defer os.Setenv(db, odb)
	os.Setenv(db, "")

	endp := newEndpoint(SessionBus, Address{"", "", ""}, nullog)
	err := endp.Dial()
	c.Check(err, NotNil)
}
