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

	id, err = ParseAppId("com.ubuntu.clock_clock_10")
	c.Assert(err, IsNil)
	c.Check(id.Package, Equals, "com.ubuntu.clock")
	c.Check(id.Application, Equals, "clock")
	c.Check(id.Version, Equals, "10")

	for _, s := range []string{"com.ubuntu.clock_clock_10_4", "com.ubuntu.clock", ""} {
		id, err = ParseAppId(s)
		c.Check(id, IsNil)
		c.Check(err, Equals, InvalidAppId)
	}
}

func (cs *clickSuite) TestInPackage(c *C) {
	c.Check(AppInPackage("com.ubuntu.clock_clock", "com.ubuntu.clock"), Equals, true)
	c.Check(AppInPackage("com.ubuntu.clock_clock_10", "com.ubuntu.clock"), Equals, true)
	c.Check(AppInPackage("com.ubuntu.clock", "com.ubuntu.clock"), Equals, false)
	c.Check(AppInPackage("bananas", "fruit"), Equals, false)
}
