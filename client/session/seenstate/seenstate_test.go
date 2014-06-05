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

package seenstate

import (
	"testing"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/protocol"
)

func TestSeenState(t *testing.T) { TestingT(t) }

type ssSuite struct {
	constructor func() (SeenState, error)
}

var _ = Suite(&ssSuite{})

func (s *ssSuite) SetUpSuite(c *C) {
	s.constructor = NewSeenState
}

func (s *ssSuite) TestAllTheLevelThings(c *C) {
	var err error
	var ss SeenState
	// checks NewSeenState returns a SeenState
	ss, err = s.constructor()
	// and that it works
	c.Assert(err, IsNil)
	// setting a couple of things, sets them
	c.Check(ss.SetLevel("this", 12), IsNil)
	c.Check(ss.SetLevel("that", 42), IsNil)
	all, err := ss.GetAllLevels()
	c.Check(err, IsNil)
	c.Check(all, DeepEquals, map[string]int64{"this": 12, "that": 42})
	// re-setting one of them, resets it
	c.Check(ss.SetLevel("this", 999), IsNil)
	all, err = ss.GetAllLevels()
	c.Check(err, IsNil)
	c.Check(all, DeepEquals, map[string]int64{"this": 999, "that": 42})
	// huzzah
}

func (s *ssSuite) TestFilterBySeen(c *C) {
	var err error
	var ss SeenState
	ss, err = s.constructor()
	// and that it works
	c.Assert(err, IsNil)
	n1 := protocol.Notification{MsgId: "m1"}
	n2 := protocol.Notification{MsgId: "m2"}
	n3 := protocol.Notification{MsgId: "m3"}
	n4 := protocol.Notification{MsgId: "m4"}
	n5 := protocol.Notification{MsgId: "m5"}

	res, err := ss.FilterBySeen([]protocol.Notification{n1, n2, n3})
	c.Assert(err, IsNil)
	// everything wasn't seen yet
	c.Check(res, DeepEquals, []protocol.Notification{n1, n2, n3})

	res, err = ss.FilterBySeen([]protocol.Notification{n1, n3, n4, n5})
	c.Assert(err, IsNil)
	// already seen n1-n3 removed
	c.Check(res, DeepEquals, []protocol.Notification{n4, n5})

	// corner case
	res, err = ss.FilterBySeen([]protocol.Notification{})
	c.Assert(err, IsNil)
	c.Assert(res, HasLen, 0)
}
