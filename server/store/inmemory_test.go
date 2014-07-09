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
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/protocol"
	help "launchpad.net/ubuntu-push/testing"
)

type inMemorySuite struct{}

var _ = Suite(&inMemorySuite{})

func (s *inMemorySuite) TestRegister(c *C) {
	sto := NewInMemoryPendingStore()

	tok1, err := sto.Register("DEV1", "app1")
	c.Assert(err, IsNil)
	tok2, err := sto.Register("DEV1", "app1")
	c.Assert(err, IsNil)
	c.Check(len(tok1), Not(Equals), 0)
	c.Check(tok1, Equals, tok2)
}

func (s *inMemorySuite) TestUnregister(c *C) {
	sto := NewInMemoryPendingStore()

	err := sto.Unregister("DEV1", "app1")
	c.Assert(err, IsNil)
}

func (s *inMemorySuite) TestGetInternalChannelIdFromToken(c *C) {
	sto := NewInMemoryPendingStore()

	tok1, err := sto.Register("DEV1", "app1")
	c.Assert(err, IsNil)
	chanId, err := sto.GetInternalChannelIdFromToken(tok1, "app1", "", "")
	c.Assert(err, IsNil)
	c.Check(chanId, Equals, UnicastInternalChannelId("DEV1", "DEV1"))
}

func (s *inMemorySuite) TestGetInternalChannelIdFromTokenFallback(c *C) {
	sto := NewInMemoryPendingStore()

	chanId, err := sto.GetInternalChannelIdFromToken("", "app1", "u1", "d1")
	c.Assert(err, IsNil)
	c.Check(chanId, Equals, UnicastInternalChannelId("u1", "d1"))
}

func (s *inMemorySuite) TestGetInternalChannelIdFromTokenErrors(c *C) {
	sto := NewInMemoryPendingStore()
	tok1, err := sto.Register("DEV1", "app1")
	c.Assert(err, IsNil)

	_, err = sto.GetInternalChannelIdFromToken(tok1, "app2", "", "")
	c.Assert(err, Equals, ErrUnauthorized)

	_, err = sto.GetInternalChannelIdFromToken("", "app2", "", "")
	c.Assert(err, Equals, ErrUnknownToken)

	_, err = sto.GetInternalChannelIdFromToken("****", "app2", "", "")
	c.Assert(err, Equals, ErrUnknownToken)
}

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
	c.Check(res, DeepEquals, []protocol.Notification(nil))
}

func (s *inMemorySuite) TestGetChannelUnfilteredEmpty(c *C) {
	sto := NewInMemoryPendingStore()

	top, res, meta, err := sto.GetChannelUnfiltered(SystemInternalChannelId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(0))
	c.Check(res, DeepEquals, []protocol.Notification(nil))
	c.Check(meta, DeepEquals, []Metadata(nil))
}

func (s *inMemorySuite) TestAppendToChannelAndGetChannelSnapshot(c *C) {
	sto := NewInMemoryPendingStore()

	notification1 := json.RawMessage(`{"a":1}`)
	notification2 := json.RawMessage(`{"a":2}`)

	muchLater := time.Now().Add(time.Minute)

	sto.AppendToChannel(SystemInternalChannelId, notification1, muchLater)
	sto.AppendToChannel(SystemInternalChannelId, notification2, muchLater)
	top, res, err := sto.GetChannelSnapshot(SystemInternalChannelId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(2))
	c.Check(res, DeepEquals, help.Ns(notification1, notification2))
}

func (s *inMemorySuite) TestAppendToUnicastChannelAndGetChannelSnapshot(c *C) {
	sto := NewInMemoryPendingStore()

	chanId := UnicastInternalChannelId("user", "dev1")
	notification1 := json.RawMessage(`{"a":1}`)
	notification2 := json.RawMessage(`{"b":2}`)

	muchLater := time.Now().Add(time.Minute)

	err := sto.AppendToUnicastChannel(chanId, "app1", notification1, "m1", muchLater)
	c.Assert(err, IsNil)
	err = sto.AppendToUnicastChannel(chanId, "app2", notification2, "m2", muchLater)
	c.Assert(err, IsNil)
	top, res, err := sto.GetChannelSnapshot(chanId)
	c.Assert(err, IsNil)
	c.Check(res, DeepEquals, []protocol.Notification{
		protocol.Notification{Payload: notification1, AppId: "app1", MsgId: "m1"},
		protocol.Notification{Payload: notification2, AppId: "app2", MsgId: "m2"},
	})
	c.Check(top, Equals, int64(0))
}

func (s *inMemorySuite) TestAppendToChannelAndGetChannelUnfiltered(c *C) {
	sto := NewInMemoryPendingStore()

	notification1 := json.RawMessage(`{"a":1}`)
	notification2 := json.RawMessage(`{"a":2}`)

	verySoon := time.Now().Add(100 * time.Millisecond)
	muchLater := time.Now().Add(time.Minute)

	sto.AppendToChannel(SystemInternalChannelId, notification1, muchLater)
	sto.AppendToChannel(SystemInternalChannelId, notification2, verySoon)

	time.Sleep(200 * time.Millisecond)

	top, res, meta, err := sto.GetChannelUnfiltered(SystemInternalChannelId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(2))
	c.Check(res, DeepEquals, help.Ns(notification1, notification2))
	c.Check(meta, DeepEquals, []Metadata{
		Metadata{Expiration: muchLater},
		Metadata{Expiration: verySoon},
	})
}

func (s *inMemorySuite) TestAppendToChannelAndGetChannelSnapshotWithExpiration(c *C) {
	sto := NewInMemoryPendingStore()

	notification1 := json.RawMessage(`{"a":1}`)
	notification2 := json.RawMessage(`{"a":2}`)

	verySoon := time.Now().Add(100 * time.Millisecond)
	muchLater := time.Now().Add(time.Minute)

	sto.AppendToChannel(SystemInternalChannelId, notification1, muchLater)
	sto.AppendToChannel(SystemInternalChannelId, notification2, verySoon)

	time.Sleep(200 * time.Millisecond)

	top, res, err := sto.GetChannelSnapshot(SystemInternalChannelId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(2))
	c.Check(res, DeepEquals, help.Ns(notification1))
}

func (s *inMemorySuite) TestScrubNop(c *C) {
	sto := NewInMemoryPendingStore()

	chanId := UnicastInternalChannelId("user", "dev1")

	err := sto.Scrub(chanId, "")
	c.Assert(err, IsNil)
}

func (s *inMemorySuite) TestScrubOnlyExpired(c *C) {
	sto := NewInMemoryPendingStore()

	chanId := UnicastInternalChannelId("user", "dev1")

	notification1 := json.RawMessage(`{"a":1}`)
	notification2 := json.RawMessage(`{"b":2}`)
	notification3 := json.RawMessage(`{"c":3}`)
	notification4 := json.RawMessage(`{"d":4}`)

	gone := time.Now().Add(-1 * time.Minute)
	muchLater1 := time.Now().Add(4 * time.Minute)
	muchLater2 := time.Now().Add(5 * time.Minute)

	err := sto.AppendToUnicastChannel(chanId, "app1", notification1, "m1", muchLater1)
	c.Assert(err, IsNil)
	err = sto.AppendToUnicastChannel(chanId, "app2", notification2, "m2", gone)
	c.Assert(err, IsNil)
	err = sto.AppendToUnicastChannel(chanId, "app1", notification3, "m3", gone)
	c.Assert(err, IsNil)
	err = sto.AppendToUnicastChannel(chanId, "app2", notification4, "m4", muchLater2)
	c.Assert(err, IsNil)

	err = sto.Scrub(chanId, "")
	c.Assert(err, IsNil)

	top, res, meta, err := sto.GetChannelUnfiltered(chanId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(0))
	c.Check(res, DeepEquals, []protocol.Notification{
		protocol.Notification{Payload: notification1, AppId: "app1", MsgId: "m1"},
		protocol.Notification{Payload: notification4, AppId: "app2", MsgId: "m4"},
	})
	c.Check(meta, DeepEquals, []Metadata{
		Metadata{Expiration: muchLater1},
		Metadata{Expiration: muchLater2},
	})
}

func (s *inMemorySuite) TestScrubApp(c *C) {
	sto := NewInMemoryPendingStore()

	chanId := UnicastInternalChannelId("user", "dev1")

	notification1 := json.RawMessage(`{"a":1}`)
	notification2 := json.RawMessage(`{"b":2}`)
	notification3 := json.RawMessage(`{"c":3}`)
	notification4 := json.RawMessage(`{"d":4}`)

	gone := time.Now().Add(-1 * time.Minute)
	muchLater := time.Now().Add(time.Minute)

	err := sto.AppendToUnicastChannel(chanId, "app1", notification1, "m1", muchLater)
	c.Assert(err, IsNil)
	err = sto.AppendToUnicastChannel(chanId, "app2", notification2, "m2", gone)
	c.Assert(err, IsNil)
	err = sto.AppendToUnicastChannel(chanId, "app1", notification3, "m3", muchLater)
	c.Assert(err, IsNil)
	err = sto.AppendToUnicastChannel(chanId, "app2", notification4, "m4", muchLater)
	c.Assert(err, IsNil)

	err = sto.Scrub(chanId, "app1")
	c.Assert(err, IsNil)

	top, res, meta, err := sto.GetChannelUnfiltered(chanId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(0))
	c.Check(res, DeepEquals, []protocol.Notification{
		protocol.Notification{Payload: notification4, AppId: "app2", MsgId: "m4"},
	})
	c.Check(meta, DeepEquals, []Metadata{
		Metadata{Expiration: muchLater},
	})
}

func (s *inMemorySuite) TestDropByMsgId(c *C) {
	sto := NewInMemoryPendingStore()

	chanId := UnicastInternalChannelId("user", "dev2")

	// nothing to do is fine
	err := sto.DropByMsgId(chanId, nil)
	c.Assert(err, IsNil)

	notification1 := json.RawMessage(`{"a":1}`)
	notification2 := json.RawMessage(`{"b":2}`)
	notification3 := json.RawMessage(`{"a":2}`)

	muchLater := time.Now().Add(time.Minute)

	err = sto.AppendToUnicastChannel(chanId, "app1", notification1, "m1", muchLater)
	c.Assert(err, IsNil)
	err = sto.AppendToUnicastChannel(chanId, "app2", notification2, "m2", muchLater)
	c.Assert(err, IsNil)
	err = sto.AppendToUnicastChannel(chanId, "app1", notification3, "m3", muchLater)
	c.Assert(err, IsNil)
	_, res, err := sto.GetChannelSnapshot(chanId)
	c.Assert(err, IsNil)

	err = sto.DropByMsgId(chanId, res[:2])
	c.Assert(err, IsNil)

	_, res, err = sto.GetChannelSnapshot(chanId)
	c.Assert(err, IsNil)
	c.Check(res, HasLen, 1)
	c.Check(res, DeepEquals, []protocol.Notification{
		protocol.Notification{Payload: notification3, AppId: "app1", MsgId: "m3"},
	})
}
