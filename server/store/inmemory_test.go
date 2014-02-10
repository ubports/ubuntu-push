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
	"encoding/json"
	// "fmt"

	. "launchpad.net/gocheck"
)

type inMemorySuite struct{}

var _ = Suite(&inMemorySuite{})

func (s *inMemorySuite) TestGetInternalChannelId(c *C) {
	sto := NewInMemoryPendingStore()

	chanId, err := sto.GetInternalChannelId("system")
	c.Check(err, IsNil)
	c.Check(chanId, Equals, SystemInternalChannelId)

	chanId, err = sto.GetInternalChannelId("blah")
	c.Check(err, Equals, ErrUnknownChannel)
	c.Check(chanId, Equals, InternalChannelId(""))
}

func (s *inMemorySuite) TestGetChannelSnapshotEmpty(c *C) {
	sto := NewInMemoryPendingStore()

	top, res, err := sto.GetChannelSnapshot(SystemInternalChannelId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(0))
	c.Check(res, DeepEquals, []json.RawMessage(nil))
}

func (s *inMemorySuite) TestAppendToChannelAndGetChannelSnapshort(c *C) {
	sto := NewInMemoryPendingStore()

	notification1 := json.RawMessage(`{"a":1}`)
	notification2 := json.RawMessage(`{"a":2}`)

	sto.AppendToChannel(SystemInternalChannelId, notification1)
	sto.AppendToChannel(SystemInternalChannelId, notification2)
	top, res, err := sto.GetChannelSnapshot(SystemInternalChannelId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(2))
	c.Check(res, DeepEquals, []json.RawMessage{notification1, notification2})
}
