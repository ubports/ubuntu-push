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

package condition

import (
	"launchpad.net/gocheck" // not into . because we have our own Not
	"testing"
)

// hook up gocheck
func Test(t *testing.T) { gocheck.TestingT(t) }

type CondSuite struct{}

var _ = gocheck.Suite(&CondSuite{})

func (s *CondSuite) TestConditionWorkTrue(c *gocheck.C) {
	cond := Work(true)
	c.Check(cond.OK(), gocheck.Equals, true)
	c.Check(cond.String(), gocheck.Equals, "Always Working.")
}

func (s *CondSuite) TestConditionWorkFalse(c *gocheck.C) {
	cond := Work(false)
	c.Check(cond.OK(), gocheck.Equals, false)
	c.Check(cond.String(), gocheck.Equals, "Never Working.")
}

func (s *CondSuite) TestConditionFail2Work(c *gocheck.C) {
	cond := Fail2Work(2)
	c.Check(cond.String(), gocheck.Equals, "Still Broken, 2 to go.")
	c.Check(cond.OK(), gocheck.Equals, false)
	c.Check(cond.String(), gocheck.Equals, "Still Broken, 1 to go.")
	c.Check(cond.OK(), gocheck.Equals, false)
	c.Check(cond.String(), gocheck.Equals, "Working.")
	c.Check(cond.OK(), gocheck.Equals, true)
	c.Check(cond.String(), gocheck.Equals, "Working.")
	c.Check(cond.OK(), gocheck.Equals, true)
	c.Check(cond.String(), gocheck.Equals, "Working.")
}

func (s *CondSuite) TestConditionNot(c *gocheck.C) {
	cond := Not(Fail2Work(1))
	c.Check(cond.String(), gocheck.Equals, "Not Still Broken, 1 to go.")
	c.Check(cond.OK(), gocheck.Equals, true)
	c.Check(cond.String(), gocheck.Equals, "Not Working.")
	c.Check(cond.OK(), gocheck.Equals, false)
}

func (s *CondSuite) TestConditionChain(c *gocheck.C) {
	cond := Chain(2, Work(true), 3, Work(false), 0, Work(true))
	c.Check(cond.String(), gocheck.Equals, "2 of Always Working. Then: 3 of Never Working. Then: 0 of Always Working.")
	c.Check(cond.OK(), gocheck.Equals, true)
	c.Check(cond.OK(), gocheck.Equals, true)
	c.Check(cond.OK(), gocheck.Equals, false)
	c.Check(cond.OK(), gocheck.Equals, false)
	c.Check(cond.OK(), gocheck.Equals, false)
	c.Check(cond.OK(), gocheck.Equals, true)
	// c.Check(cond.OK(), gocheck.Equals, true)
	// c.Check(cond.OK(), gocheck.Equals, true)
	// c.Check(cond.OK(), gocheck.Equals, true)
}
