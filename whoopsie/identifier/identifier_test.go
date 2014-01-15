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

// TestGenerate checks that Generate() does not fail, and returns a
// 128-byte string.
func (s *IdentifierSuite) TestGenerate(c *C) {
	id := New()

	c.Check(id.Generate(), Equals, nil)
	c.Check(id.String(), HasLen, 128)
}

// TestIdentifierInterface checks that Identifier implements Id.
func (s *IdentifierSuite) TestIdentifierInterface(c *C) {
	_ = []Id{New()}
}
