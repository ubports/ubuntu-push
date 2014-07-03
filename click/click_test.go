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
	ver := u.getVersion("com.ubuntu.clock")
	if ver == "" {
		c.Skip("no com.ubuntu.clock pkg installed")
	}
	c.Check(u.HasPackage("com.ubuntu.clock_clock"), Equals, true)
	c.Check(u.HasPackage("com.ubuntu.clock_clock_" + ver), Equals, true)
}
