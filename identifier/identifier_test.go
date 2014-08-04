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

package identifier

import (
	. "launchpad.net/gocheck"
	"testing"
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type IdentifierSuite struct{}

var _ = Suite(&IdentifierSuite{})

// TestNew checks that New does not fail, and returns a
// 32-byte string.
func (s *IdentifierSuite) TestNew(c *C) {
	id, err := New()
	c.Check(err, IsNil)
	c.Check(id.String(), HasLen, 32)
}

// TestNewFail checks that when we can't read the machine-id
// file the error is propagated
func (s *IdentifierSuite) TestNewFail(c *C) {
	// replace the machine-id file path
	machineIdPath = "/var/lib/dbus/no-such-file"
	id, err := New()
	c.Check(err, NotNil)
	c.Check(err.Error(), Equals, "Failed to read the machine id: open /var/lib/dbus/no-such-file: no such file or directory")
	c.Check(id.String(), HasLen, 0)
}

// TestIdentifierInterface checks that Identifier implements Id.
func (s *IdentifierSuite) TestIdentifierInterface(c *C) {
	id, _ := New()
	_ = []Id{id}
}
