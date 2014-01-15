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

package broker

import (
	"encoding/json"
	"errors"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/store"
	helpers "launchpad.net/ubuntu-push/testing"
	// "log"
	"time"
)

type simpleSuite struct{}

var _ = Suite(&simpleSuite{})

type testBrokerConfig struct{}

func (tbc *testBrokerConfig) SessionQueueSize() uint {
	return 10
}

func (tbc *testBrokerConfig) BrokerQueueSize() uint {
	return 5
}

func (s *simpleSuite) TestNew(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := NewSimpleBroker(sto, &testBrokerConfig{}, nil)
	c.Check(cap(b.sessionCh), Equals, 5)
	c.Check(len(b.registry), Equals, 0)
	c.Check(b.sto, Equals, sto)
}

func (s *simpleSuite) TestStartStop(c *C) {
	b := NewSimpleBroker(nil, &testBrokerConfig{}, nil)
	b.Start()
	c.Check(b.running, Equals, true)
	b.Start()
	b.Stop()
	c.Check(b.running, Equals, false)
	b.Stop()
}

func (s *simpleSuite) TestRegistration(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := NewSimpleBroker(sto, &testBrokerConfig{}, nil)
	b.Start()
	defer b.Stop()
	sess, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1", Levels: map[string]int64{"0": 5}})
	c.Assert(err, IsNil)
	c.Assert(b.registry["dev-1"], Equals, sess)
	c.Assert(sess.DeviceId(), Equals, "dev-1")
	c.Check(sess.(*simpleBrokerSession).levels, DeepEquals, map[store.InternalChannelId]int64{
		store.SystemInternalChannelId: 5,
	})
	b.Unregister(sess)
	// just to make sure the unregister was processed
	_, err = b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: ""})
	c.Assert(err, IsNil)
	c.Check(b.registry["dev-1"], IsNil)
}

func (s *simpleSuite) TestRegistrationBrokenLevels(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := NewSimpleBroker(sto, &testBrokerConfig{}, nil)
	b.Start()
	defer b.Stop()
	_, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1", Levels: map[string]int64{"z": 5}})
	c.Check(err, FitsTypeOf, &ErrAbort{})
}

func (s *simpleSuite) TestFeedPending(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M"}`)
	sto.AppendToChannel(store.SystemInternalChannelId, notification1)
	b := NewSimpleBroker(sto, &testBrokerConfig{}, nil)
	sess := &simpleBrokerSession{
		exchanges: make(chan Exchange, 1),
	}
	b.feedPending(sess)
	c.Assert(len(sess.exchanges), Equals, 1)
	exchg1 := <-sess.exchanges
	c.Check(exchg1, DeepEquals, &simpleBroadcastExchange{
		chanId:               store.SystemInternalChannelId,
		topLevel:             1,
		notificationPayloads: []json.RawMessage{notification1},
	})
}

func (s *simpleSuite) TestFeedPendingNop(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M"}`)
	sto.AppendToChannel(store.SystemInternalChannelId, notification1)
	b := NewSimpleBroker(sto, &testBrokerConfig{}, nil)
	sess := &simpleBrokerSession{
		exchanges: make(chan Exchange, 1),
		levels: map[store.InternalChannelId]int64{
			store.SystemInternalChannelId: 1,
		},
	}
	b.feedPending(sess)
	c.Assert(len(sess.exchanges), Equals, 0)
}

func (s *simpleSuite) TestRegistrationFeedPending(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M"}`)
	sto.AppendToChannel(store.SystemInternalChannelId, notification1)
	b := NewSimpleBroker(sto, &testBrokerConfig{}, nil)
	b.Start()
	defer b.Stop()
	sess, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	c.Check(len(sess.(*simpleBrokerSession).exchanges), Equals, 1)
}

func (s *simpleSuite) TestRegistrationFeedPendingError(c *C) {
	buf := &helpers.SyncedLogBuffer{}
	logger := logger.NewSimpleLogger(buf, "error")
	sto := &testFailingStore{}
	b := NewSimpleBroker(sto, &testBrokerConfig{}, logger)
	b.Start()
	defer b.Stop()
	_, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	// but
	c.Check(buf.String(), Matches, ".*ERROR unsuccessful feed pending, get channel snapshot for 0: get channel snapshot fail\n")
}

func (s *simpleSuite) TestRegistrationLastWins(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := NewSimpleBroker(sto, &testBrokerConfig{}, nil)
	b.Start()
	defer b.Stop()
	sess1, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	sess2, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	c.Assert(b.registry["dev-1"], Equals, sess2)
	b.Unregister(sess1)
	// just to make sure the unregister was processed
	_, err = b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: ""})
	c.Assert(err, IsNil)
	c.Check(b.registry["dev-1"], Equals, sess2)
}

func (s *simpleSuite) TestBroadcastExchange(c *C) {
	sess := &simpleBrokerSession{
		levels: map[store.InternalChannelId]int64{},
	}
	exchg := &simpleBroadcastExchange{
		chanId:   store.SystemInternalChannelId,
		topLevel: 3,
		notificationPayloads: []json.RawMessage{
			json.RawMessage(`{"a":"x"}`),
			json.RawMessage(`{"a":"y"}`),
		},
	}
	inMsg, outMsg, err := exchg.Prepare(sess)
	c.Assert(err, IsNil)
	// check
	marshalled, err := json.Marshal(inMsg)
	c.Assert(err, IsNil)
	c.Check(string(marshalled), Equals, `{"T":"broadcast","ChanId":"0","TopLevel":3,"Payloads":[{"a":"x"},{"a":"y"}]}`)
	err = json.Unmarshal([]byte(`{"T":"ack"}`), outMsg)
	c.Assert(err, IsNil)
	err = exchg.Acked(sess)
	c.Assert(err, IsNil)
	c.Check(sess.levels[store.SystemInternalChannelId], Equals, int64(3))
}

func (s *simpleSuite) TestBroadcastExchangeAckMismatch(c *C) {
	sess := &simpleBrokerSession{
		levels: map[store.InternalChannelId]int64{},
	}
	exchg := &simpleBroadcastExchange{
		chanId:   store.SystemInternalChannelId,
		topLevel: 3,
		notificationPayloads: []json.RawMessage{
			json.RawMessage(`{"a":"y"}`),
		},
	}
	inMsg, outMsg, err := exchg.Prepare(sess)
	c.Assert(err, IsNil)
	// check
	marshalled, err := json.Marshal(inMsg)
	c.Assert(err, IsNil)
	c.Check(string(marshalled), Equals, `{"T":"broadcast","ChanId":"0","TopLevel":3,"Payloads":[{"a":"y"}]}`)
	err = json.Unmarshal([]byte(`{}`), outMsg)
	c.Assert(err, IsNil)
	err = exchg.Acked(sess)
	c.Assert(err, Not(IsNil))
	c.Check(sess.levels[store.SystemInternalChannelId], Equals, int64(0))
}

func (s *simpleSuite) TestFilterByLevel(c *C) {
	payloads := []json.RawMessage{
		json.RawMessage(`{"a": 3}`),
		json.RawMessage(`{"a": 4}`),
		json.RawMessage(`{"a": 5}`),
	}
	res := filterByLevel(5, 5, payloads)
	c.Check(len(res), Equals, 0)
	res = filterByLevel(4, 5, payloads)
	c.Check(len(res), Equals, 1)
	c.Check(res[0], DeepEquals, json.RawMessage(`{"a": 5}`))
	res = filterByLevel(3, 5, payloads)
	c.Check(len(res), Equals, 2)
	c.Check(res[0], DeepEquals, json.RawMessage(`{"a": 4}`))
	res = filterByLevel(2, 5, payloads)
	c.Check(len(res), Equals, 3)
	res = filterByLevel(1, 5, payloads)
	c.Check(len(res), Equals, 3)
}

func (s *simpleSuite) TestBroadcastExchangeFilterByLevel(c *C) {
	sess := &simpleBrokerSession{
		levels: map[store.InternalChannelId]int64{
			store.SystemInternalChannelId: 2,
		},
	}
	exchg := &simpleBroadcastExchange{
		chanId:   store.SystemInternalChannelId,
		topLevel: 3,
		notificationPayloads: []json.RawMessage{
			json.RawMessage(`{"a":"x"}`),
			json.RawMessage(`{"a":"y"}`),
		},
	}
	inMsg, outMsg, err := exchg.Prepare(sess)
	c.Assert(err, IsNil)
	// check
	marshalled, err := json.Marshal(inMsg)
	c.Assert(err, IsNil)
	c.Check(string(marshalled), Equals, `{"T":"broadcast","ChanId":"0","TopLevel":3,"Payloads":[{"a":"y"}]}`)
	err = json.Unmarshal([]byte(`{"T":"ack"}`), outMsg)
	c.Assert(err, IsNil)
	err = exchg.Acked(sess)
	c.Assert(err, IsNil)
}

func (s *simpleSuite) TestBroadcast(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M"}`)
	sto.AppendToChannel(store.SystemInternalChannelId, notification1)
	b := NewSimpleBroker(sto, &testBrokerConfig{}, nil)
	b.Start()
	defer b.Stop()
	sess1, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	sess2, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-2"})
	c.Assert(err, IsNil)
	b.Broadcast(store.SystemInternalChannelId)
	select {
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to get broadcast exchange")
	case exchg1 := <-sess1.SessionChannel():
		c.Check(exchg1, DeepEquals, &simpleBroadcastExchange{
			chanId:               store.SystemInternalChannelId,
			topLevel:             1,
			notificationPayloads: []json.RawMessage{notification1},
		})
	}
	select {
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to get broadcast exchange")
	case exchg2 := <-sess2.SessionChannel():
		c.Check(exchg2, DeepEquals, &simpleBroadcastExchange{
			chanId:               store.SystemInternalChannelId,
			topLevel:             1,
			notificationPayloads: []json.RawMessage{notification1},
		})
	}
}

type testFailingStore struct {
	store.InMemoryPendingStore
	countdownToFail int
}

func (sto *testFailingStore) GetChannelSnapshot(chanId store.InternalChannelId) (int64, []json.RawMessage, error) {
	if sto.countdownToFail == 0 {
		return 0, nil, errors.New("get channel snapshot fail")
	}
	sto.countdownToFail--
	return 0, nil, nil
}

func (s *simpleSuite) TestBroadcastFail(c *C) {
	buf := &helpers.SyncedLogBuffer{Written: make(chan bool, 1)}
	logger := logger.NewSimpleLogger(buf, "error")
	sto := &testFailingStore{countdownToFail: 1}
	b := NewSimpleBroker(sto, &testBrokerConfig{}, logger)
	b.Start()
	defer b.Stop()
	_, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	b.Broadcast(store.SystemInternalChannelId)
	select {
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to log error")
	case <-buf.Written:
	}
	c.Check(buf.String(), Matches, ".*ERROR unsuccessful broadcast, get channel snapshot for 0: get channel snapshot fail\n")
}
