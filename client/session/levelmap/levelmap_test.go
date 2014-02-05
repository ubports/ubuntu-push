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

type lmSuite struct{}

var _ = Suite(&lmSuite{})

func (cs *lmSuite) TestAllTheThings(c *C) {
	// checks NewLevelMap returns a LevelMap
	var lm LevelMap = NewLevelMap()
	// setting a couple of things, sets them
	lm.Set("this", 12)
	lm.Set("that", 42)
	c.Check(lm.GetAll(), DeepEquals, map[string]int64{"this": 12, "that": 42})
	// re-setting one of them, resets it
	lm.Set("this", 999)
	c.Check(lm.GetAll(), DeepEquals, map[string]int64{"this": 999, "that": 42})
	// huzzah
}
