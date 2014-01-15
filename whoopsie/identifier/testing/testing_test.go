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

package testing

import (
	. "launchpad.net/gocheck"
	"testing"
	identifier ".."
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type IdentifierSuite struct{}
var _ = Suite(&IdentifierSuite{})

// TestSettableDefaultValueVisible tests SettableIdentifier's default
// value is notable.
func (s *IdentifierSuite) TestSettableDefaultValueVisible(c *C) {
	id := Settable()
	c.Check(id.String(), Equals, "<Settable>")
}

// TestSettableSets tests SettableIdentifier is settable.
func (s *IdentifierSuite) TestSettableSets(c *C) {
	id := Settable()
	id.Set("hello")
	c.Check(id.String(), Equals, "hello")
}

// TestSettableGenerateDoesNotFail tests SettableIdentifier's Generate
// does not fail.
func (s *IdentifierSuite) TestSettableGenerateDoesNotFail(c *C) {
	id := Settable()
	c.Check(id.Generate(), Equals, nil)
}

// TestFailingFails tests FailingIdentifier fails.
func (s *IdentifierSuite) TestFailingFails(c *C) {
	id := Failing()
	c.Check(id.Generate(), Not(Equals), nil)
}

// TestFailingStringNotEmpty tests FailingIdentifier still has a
// non-empty string.
func (s *IdentifierSuite) TestFailingStringNotEmpty(c *C) {
	id := Failing()
	c.Check(id.String(), Equals, "<Failing>")
}

// TestIdentifierInterface tests FailingIdentifier and
// SettableIdentifier implement Id.
func (s *IdentifierSuite) TestIdentifierInterface(c *C) {
	_ = []identifier.Id{Failing(), Settable()}
}
