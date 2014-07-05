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
	c.Check(id.Application, Equals, "clock")
	c.Check(id.Version, Equals, "")
	c.Check(id.Click, Equals, true)

	id, err = ParseAppId("com.ubuntu.clock_clock_10")
	c.Assert(err, IsNil)
	c.Check(id.Package, Equals, "com.ubuntu.clock")
	c.Check(id.Application, Equals, "clock")
	c.Check(id.Version, Equals, "10")
	c.Check(id.Click, Equals, true)

	for _, s := range []string{"com.ubuntu.clock_clock_10_4", "com.ubuntu.clock", ""} {
		id, err = ParseAppId(s)
		c.Check(id, IsNil)
		c.Check(err, Equals, ErrInvalidAppId)
	}
}

func (cs *clickSuite) TestParseAppIdLegacy(c *C) {
	id, err := ParseAppId("python3.4")
	c.Assert(err, IsNil)
	c.Check(id.Package, Equals, "python3.4")
	c.Check(id.Application, Equals, "python3.4")
	c.Check(id.Version, Equals, "")
	c.Check(id.Click, Equals, false)
}

func (cs *clickSuite) TestInPackage(c *C) {
	c.Check(AppInPackage("com.ubuntu.clock_clock", "com.ubuntu.clock"), Equals, true)
	c.Check(AppInPackage("com.ubuntu.clock_clock_10", "com.ubuntu.clock"), Equals, true)
	c.Check(AppInPackage("com.ubuntu.clock", "com.ubuntu.clock"), Equals, false)
	c.Check(AppInPackage("bananas", "fruit"), Equals, false)
}

func (cs *clickSuite) TestInPackageLegacy(c *C) {
	c.Check(AppInPackage("python3.4", "python3.4"), Equals, true)
}

func (s *clickSuite) TestUser(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	c.Assert(u, NotNil)
}

func (s *clickSuite) TestHasPackageNegative(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	c.Check(u.HasPackage("com.foo.bar"), Equals, false)
	c.Check(u.HasPackage("com.foo.bar_baz"), Equals, false)
}

func (s *clickSuite) TestHasPackageVersionNegative(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	c.Check(u.HasPackage("com.ubuntu.clock_clock_1000.0"), Equals, false)
}

func (s *clickSuite) TestHasPackageClock(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	ver := u.ccu.CGetVersion("com.ubuntu.clock")
	if ver == "" {
		c.Skip("no com.ubuntu.clock pkg installed")
	}
	c.Check(u.HasPackage("com.ubuntu.clock_clock"), Equals, true)
	c.Check(u.HasPackage("com.ubuntu.clock_clock_"+ver), Equals, true)
}

func (s *clickSuite) TestHasPackageLegacy(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	c.Check(u.HasPackage("python3.4"), Equals, true)
}
