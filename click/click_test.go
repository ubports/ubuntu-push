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

package click

import (
	"testing"

	. "launchpad.net/gocheck"
)

func TestClick(t *testing.T) { TestingT(t) }

type clickSuite struct{}

var _ = Suite(&clickSuite{})

func (cs *clickSuite) TestParseAppId(c *C) {
	id, err := ParseAppId("com.ubuntu.clock_clock")
	c.Assert(err, IsNil)
	c.Check(id.Package, Equals, "com.ubuntu.clock")
	c.Check(id.InPackage("com.ubuntu.clock"), Equals, true)
	c.Check(id.Application, Equals, "clock")
	c.Check(id.Version, Equals, "")
	c.Check(id.Click, Equals, true)
	c.Check(id.Original(), Equals, "com.ubuntu.clock_clock")

	id, err = ParseAppId("com.ubuntu.clock_clock_10")
	c.Assert(err, IsNil)
	c.Check(id.Package, Equals, "com.ubuntu.clock")
	c.Check(id.InPackage("com.ubuntu.clock"), Equals, true)
	c.Check(id.Application, Equals, "clock")
	c.Check(id.Version, Equals, "10")
	c.Check(id.Click, Equals, true)
	c.Check(id.Original(), Equals, "com.ubuntu.clock_clock_10")
	c.Check(id.Versioned(), Equals, "com.ubuntu.clock_clock_10")
	c.Check(id.DesktopId(), Equals, "com.ubuntu.clock_clock_10.desktop")

	for _, s := range []string{"com.ubuntu.clock_clock_10_4", "com.ubuntu.clock", ""} {
		id, err = ParseAppId(s)
		c.Check(id, IsNil)
		c.Check(err, Equals, ErrInvalidAppId)
	}
}

func (cs *clickSuite) TestParseAppIdLegacy(c *C) {
	id, err := ParseAppId("_python3.4")
	c.Assert(err, IsNil)
	c.Check(id.Package, Equals, "")
	c.Check(id.InPackage(""), Equals, true)
	c.Check(id.Application, Equals, "python3.4")
	c.Check(id.Version, Equals, "")
	c.Check(id.Click, Equals, false)
	c.Check(id.Original(), Equals, "_python3.4")
	c.Check(id.Versioned(), Equals, "python3.4")
	c.Check(id.DesktopId(), Equals, "python3.4.desktop")

	for _, s := range []string{"_.foo", "_foo/", "_/foo"} {
		id, err = ParseAppId(s)
		c.Check(id, IsNil)
		c.Check(err, Equals, ErrInvalidAppId)
	}
}

func (s *clickSuite) TestUser(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	c.Assert(u, NotNil)
}

func (s *clickSuite) TestInstalledNegative(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	id, err := ParseAppId("com.foo.bar_baz")
	c.Assert(err, IsNil)
	c.Check(u.Installed(id, false), Equals, false)
}

func (s *clickSuite) TestInstalledVersionNegative(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	id, err := ParseAppId("com.ubuntu.clock_clock_1000.0")
	c.Assert(err, IsNil)
	c.Check(u.Installed(id, false), Equals, false)
}

func (s *clickSuite) TestInstalledClock(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	ver := u.ccu.CGetVersion("com.ubuntu.clock")
	if ver == "" {
		c.Skip("no com.ubuntu.clock pkg installed")
	}
	id, err := ParseAppId("com.ubuntu.clock_clock")
	c.Assert(err, IsNil)
	c.Check(u.Installed(id, false), Equals, true)
	id, err = ParseAppId("com.ubuntu.clock_clock_" + ver)
	c.Assert(err, IsNil)
	c.Check(u.Installed(id, false), Equals, true)

	id, err = ParseAppId("com.ubuntu.clock_clock_10" + ver)
	c.Assert(err, IsNil)
	c.Check(u.Installed(id, false), Equals, false)

	// setVersion
	id, err = ParseAppId("com.ubuntu.clock_clock")
	c.Assert(err, IsNil)
	c.Check(u.Installed(id, true), Equals, true)
	c.Check(id.Version, Equals, ver)
}

func (s *clickSuite) TestInstalledLegacy(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	id, err := ParseAppId("_python3.4")
	c.Assert(err, IsNil)
	c.Check(u.Installed(id, false), Equals, true)
}

func (s *clickSuite) TestParseAndVerifyAppId(c *C) {
	u, err := User()
	c.Assert(err, IsNil)

	id, err := ParseAndVerifyAppId("_.foo", nil)
	c.Assert(err, Equals, ErrInvalidAppId)
	c.Check(id, IsNil)

	id, err = ParseAndVerifyAppId("com.foo.bar_baz", nil)
	c.Assert(err, IsNil)
	c.Check(id.Click, Equals, true)
	c.Check(id.Application, Equals, "baz")

	id, err = ParseAndVerifyAppId("_non-existent-app", u)
	c.Assert(err, Equals, ErrMissingAppId)
	c.Check(id, IsNil)

}
