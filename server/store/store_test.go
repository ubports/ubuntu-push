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

package store

import (
	// "fmt"
	"testing"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/protocol"
)

func TestStore(t *testing.T) { TestingT(t) }

type storeSuite struct{}

var _ = Suite(&storeSuite{})

func (s *storeSuite) TestInternalChannelIdToHex(c *C) {
	c.Check(InternalChannelIdToHex(SystemInternalChannelId), Equals, protocol.SystemChannelId)
	c.Check(InternalChannelIdToHex(InternalChannelId("Bf1c9bf7096084cb2a154979ce00c7f50")), Equals, "f1c9bf7096084cb2a154979ce00c7f50")
	c.Check(func() { InternalChannelIdToHex(InternalChannelId("U")) }, PanicMatches, "InternalChannelIdToHex is for broadcast channels")
}

func (s *storeSuite) TestHexToInternalChannelId(c *C) {
	i0, err := HexToInternalChannelId("0")
	c.Check(err, IsNil)
	c.Check(i0, Equals, SystemInternalChannelId)
	i1, err := HexToInternalChannelId("00000000000000000000000000000000")
	c.Check(err, IsNil)
	c.Check(i1, Equals, SystemInternalChannelId)
	c.Check(i1.BroadcastChannel(), Equals, true)
	i2, err := HexToInternalChannelId("f1c9bf7096084cb2a154979ce00c7f50")
	c.Check(err, IsNil)
	c.Check(i2.BroadcastChannel(), Equals, true)
	c.Check(i2, Equals, InternalChannelId("Bf1c9bf7096084cb2a154979ce00c7f50"))
	_, err = HexToInternalChannelId("01")
	c.Check(err, Equals, ErrExpected128BitsHexRepr)
	_, err = HexToInternalChannelId("abceddddddddddddddddzeeeeeeeeeee")
	c.Check(err, Equals, ErrExpected128BitsHexRepr)
	_, err = HexToInternalChannelId("f1c9bf7096084cb2a154979ce00c7f50ff")
	c.Check(err, Equals, ErrExpected128BitsHexRepr)
}

func (s *storeSuite) TestUnicastInternalChannelId(c *C) {
	chanId := UnicastInternalChannelId("user1", "dev2")
	c.Check(chanId.BroadcastChannel(), Equals, false)
	c.Check(chanId.UnicastChannel(), Equals, true)
	u, d := chanId.UnicastUserAndDevice()
	c.Check(u, Equals, "user1")
	c.Check(d, Equals, "dev2")
	c.Check(func() { SystemInternalChannelId.UnicastUserAndDevice() }, PanicMatches, "UnicastUserAndDevice is for unicast channels")
}

func (s *storeSuite) TestDropByMsgId(c *C) {
	orig := []protocol.Notification{
		protocol.Notification{MsgId: "a"},
		protocol.Notification{MsgId: "b"},
		protocol.Notification{MsgId: "c"},
		protocol.Notification{MsgId: "d"},
	}
	// removing the continuous head
	res := DropByMsgId(orig, orig[:3])
	c.Check(res, DeepEquals, orig[3:])

	// random removal
	res = DropByMsgId(orig, orig[1:2])
	c.Check(res, DeepEquals, []protocol.Notification{
		protocol.Notification{MsgId: "a"},
		protocol.Notification{MsgId: "c"},
		protocol.Notification{MsgId: "d"},
	})

	// looks like removing the continuous head, but it isn't
	res = DropByMsgId(orig, []protocol.Notification{
		protocol.Notification{MsgId: "a"},
		protocol.Notification{MsgId: "c"},
		protocol.Notification{MsgId: "d"},
	})
	c.Check(res, DeepEquals, []protocol.Notification{
		protocol.Notification{MsgId: "b"},
	})

}
