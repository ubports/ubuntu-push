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
	c.Check(InternalChannelIdToHex(InternalChannelId("B\xf1\xc9\xbf\x70\x96\x08\x4c\xb2\xa1\x54\x97\x9c\xe0\x0c\x7f\x50")), Equals, "f1c9bf7096084cb2a154979ce00c7f50")
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
	c.Check(i2, Equals, InternalChannelId("B\xf1\xc9\xbf\x70\x96\x08\x4c\xb2\xa1\x54\x97\x9c\xe0\x0c\x7f\x50"))
	_, err = HexToInternalChannelId("01")
	c.Check(err, Equals, ErrExpected128BitsHexRepr)
	_, err = HexToInternalChannelId("abceddddddddddddddddzeeeeeeeeeee")
	c.Check(err, Equals, ErrExpected128BitsHexRepr)
	_, err = HexToInternalChannelId("f1c9bf7096084cb2a154979ce00c7f50ff")
	c.Check(err, Equals, ErrExpected128BitsHexRepr)
}
