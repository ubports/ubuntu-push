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

// Package testsuite contains a common test suite for brokers.
package testsuite

import (
	"encoding/json"
	"errors"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/broker"
	"launchpad.net/ubuntu-push/server/broker/testing"
	"launchpad.net/ubuntu-push/server/store"
	helpers "launchpad.net/ubuntu-push/testing"
	// "log"
	"time"
)

// The expected interface for tested brokers.
type FullBroker interface {
	broker.Broker
	broker.BrokerSending
	Start()
	Stop()
	Running() bool
}

// The common brokers' test suite.
type CommonBrokerSuite struct {
	// Build the broker for testing.
	MakeBroker func(store.PendingStore, broker.BrokerConfig, logger.Logger) FullBroker
	// Let us get to a session under the broker.
	RevealSession func(broker.Broker, string) broker.BrokerSession
	// Let us get to a broker.BroadcastExchange from an Exchange.
	RevealBroadcastExchange func(broker.Exchange) *broker.BroadcastExchange
	// private
	testlog *helpers.TestLogger
}

func (s *CommonBrokerSuite) SetUpTest(c *C) {
	s.testlog = helpers.NewTestLogger(c, "error")
}

var testBrokerConfig = &testing.TestBrokerConfig{10, 5}

func (s *CommonBrokerSuite) TestSanity(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	c.Check(s.RevealSession(b, "FOO"), IsNil)
}

func (s *CommonBrokerSuite) TestStartStop(c *C) {
	b := s.MakeBroker(nil, testBrokerConfig, nil)
	b.Start()
	c.Check(b.Running(), Equals, true)
	b.Start()
	b.Stop()
	c.Check(b.Running(), Equals, false)
	b.Stop()
}

func (s *CommonBrokerSuite) TestRegistration(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	sess, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1", Levels: map[string]int64{"0": 5}})
	c.Assert(err, IsNil)
	c.Assert(s.RevealSession(b, "dev-1"), Equals, sess)
	c.Assert(sess.DeviceIdentifier(), Equals, "dev-1")
	c.Assert(sess.ExchangeScratchArea(), Not(IsNil))
	c.Check(sess.Levels(), DeepEquals, broker.LevelsMap(map[store.InternalChannelId]int64{
		store.SystemInternalChannelId: 5,
	}))
	b.Unregister(sess)
	// just to make sure the unregister was processed
	_, err = b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: ""})
	c.Assert(err, IsNil)
	c.Check(s.RevealSession(b, "dev-1"), IsNil)
}

func (s *CommonBrokerSuite) TestRegistrationBrokenLevels(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	_, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1", Levels: map[string]int64{"z": 5}})
	c.Check(err, FitsTypeOf, &broker.ErrAbort{})
}

func (s *CommonBrokerSuite) TestRegistrationFeedPending(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M"}`)
	sto.AppendToChannel(store.SystemInternalChannelId, notification1)
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	sess, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	c.Check(len(sess.SessionChannel()), Equals, 1)
}

func (s *CommonBrokerSuite) TestRegistrationFeedPendingError(c *C) {
	sto := &testFailingStore{}
	b := s.MakeBroker(sto, testBrokerConfig, s.testlog)
	b.Start()
	defer b.Stop()
	_, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	// but
	c.Check(s.testlog.Captured(), Matches, "ERROR unsuccessful feed pending, get channel snapshot for 0: get channel snapshot fail\n")
}

func (s *CommonBrokerSuite) TestRegistrationLastWins(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	sess1, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	sess2, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	c.Assert(s.RevealSession(b, "dev-1"), Equals, sess2)
	b.Unregister(sess1)
	// just to make sure the unregister was processed
	_, err = b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: ""})
	c.Assert(err, IsNil)
	c.Check(s.RevealSession(b, "dev-1"), Equals, sess2)
}

func (s *CommonBrokerSuite) TestBroadcast(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M"}`)
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	sess1, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	sess2, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-2"})
	c.Assert(err, IsNil)
	// add notification to channel *after* the registrations
	sto.AppendToChannel(store.SystemInternalChannelId, notification1)
	b.Broadcast(store.SystemInternalChannelId)
	select {
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to get broadcast exchange")
	case exchg1 := <-sess1.SessionChannel():
		c.Check(s.RevealBroadcastExchange(exchg1), DeepEquals, &broker.BroadcastExchange{
			ChanId:               store.SystemInternalChannelId,
			TopLevel:             1,
			NotificationPayloads: []json.RawMessage{notification1},
		})
	}
	select {
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to get broadcast exchange")
	case exchg2 := <-sess2.SessionChannel():
		c.Check(s.RevealBroadcastExchange(exchg2), DeepEquals, &broker.BroadcastExchange{
			ChanId:               store.SystemInternalChannelId,
			TopLevel:             1,
			NotificationPayloads: []json.RawMessage{notification1},
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

func (s *CommonBrokerSuite) TestBroadcastFail(c *C) {
	s.testlog.Written = make(chan bool, 1)
	sto := &testFailingStore{countdownToFail: 1}
	b := s.MakeBroker(sto, testBrokerConfig, s.testlog)
	b.Start()
	defer b.Stop()
	_, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"})
	c.Assert(err, IsNil)
	b.Broadcast(store.SystemInternalChannelId)
	select {
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to log error")
	case <-s.testlog.Written:
	}
	c.Check(s.testlog.Captured(), Matches, "ERROR unsuccessful broadcast, get channel snapshot for 0: get channel snapshot fail\n")
}
