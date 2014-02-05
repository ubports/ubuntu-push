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
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"os"
)

type EndpointSuite struct {
	log logger.Logger
}

var _ = Suite(&EndpointSuite{})

func (s *EndpointSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
	s.log.Debugf("---")
}

// TODO: this is going to remain empty until go-dbus grows some
// testing amenities (already talked about it with jamesh)

// Tests that we can connect to the *actual* system bus.
// XXX maybe connect to a mock/fake/etc bus?
func (s *EndpointSuite) TestDial(c *C) {
	endp := newEndpoint(SystemBus, Address{"", "", ""}, s.log)
	c.Assert(endp.bus, IsNil)
	err := endp.Dial()
	c.Assert(err, IsNil)
	defer endp.Close() // yes, a second close. On purpose.
	c.Assert(endp.bus, NotNil)
	endp.Close()              // the first close. If you're counting right.
	c.Assert(endp.bus, IsNil) // Close cleans up
}

// Test that if we try to connect to the session bus when no session
// bus is available, we get a reasonable result (i.e., an error).
func (s *EndpointSuite) TestDialCanFail(c *C) {
	db := "DBUS_SESSION_BUS_ADDRESS"
	odb := os.Getenv(db)
	defer os.Setenv(db, odb)
	os.Setenv(db, "")

	endp := newEndpoint(SessionBus, Address{"", "", ""}, s.log)
	err := endp.Dial()
	c.Check(err, NotNil)
}
