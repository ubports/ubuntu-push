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

package levelmap

import (
	. "launchpad.net/gocheck"
	"testing"
)

func TestLevelMap(t *testing.T) { TestingT(t) }

type lmSuite struct {
	constructor func() (LevelMap, error)
}

var _ = Suite(&lmSuite{})

func (s *lmSuite) SetUpSuite(c *C) {
	s.constructor = NewLevelMap
}

func (s *lmSuite) TestAllTheThings(c *C) {
	var err error
	var lm LevelMap
	// checks NewLevelMap returns a LevelMap
	lm, err = s.constructor()
	// and that it works
	c.Assert(err, IsNil)
	// setting a couple of things, sets them
	c.Check(lm.Set("this", 12), IsNil)
	c.Check(lm.Set("that", 42), IsNil)
	all, err := lm.GetAll()
	c.Check(err, IsNil)
	c.Check(all, DeepEquals, map[string]int64{"this": 12, "that": 42})
	// re-setting one of them, resets it
	c.Check(lm.Set("this", 999), IsNil)
	all, err = lm.GetAll()
	c.Check(err, IsNil)
	c.Check(all, DeepEquals, map[string]int64{"this": 999, "that": 42})
	// huzzah
}
