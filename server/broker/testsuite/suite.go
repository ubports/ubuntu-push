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
	// "log"
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/broker"
	"launchpad.net/ubuntu-push/server/broker/testing"
	"launchpad.net/ubuntu-push/server/store"
	help "launchpad.net/ubuntu-push/testing"
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
	// Let us get to a broker.UnicastExchange from an Exchange.
	RevealUnicastExchange func(broker.Exchange) *broker.UnicastExchange
	// private
	testlog *help.TestLogger
}

func (s *CommonBrokerSuite) SetUpTest(c *C) {
	s.testlog = help.NewTestLogger(c, "error")
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
	sess, err := b.Register(&protocol.ConnectMsg{
		Type:     "connect",
		DeviceId: "dev-1",
		Levels:   map[string]int64{"0": 5},
		Info: map[string]interface{}{
			"device":  "model",
			"channel": "daily",
		},
	}, "s1")
	c.Assert(err, IsNil)
	c.Assert(s.RevealSession(b, "dev-1"), Equals, sess)
	c.Assert(sess.DeviceIdentifier(), Equals, "dev-1")
	c.Check(sess.DeviceImageModel(), Equals, "model")
	c.Check(sess.DeviceImageChannel(), Equals, "daily")
	c.Assert(sess.ExchangeScratchArea(), Not(IsNil))
	c.Check(sess.Levels(), DeepEquals, broker.LevelsMap(map[store.InternalChannelId]int64{
		store.SystemInternalChannelId: 5,
	}))
	b.Unregister(sess)
	// just to make sure the unregister was processed
	_, err = b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: ""}, "s2")
	c.Assert(err, IsNil)
	c.Check(s.RevealSession(b, "dev-1"), IsNil)
}

func (s *CommonBrokerSuite) TestRegistrationBrokenLevels(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	_, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1", Levels: map[string]int64{"z": 5}}, "s1")
	c.Check(err, FitsTypeOf, &broker.ErrAbort{})
}

func (s *CommonBrokerSuite) TestRegistrationInfoErrors(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	info := map[string]interface{}{
		"device": -1,
	}
	_, err := b.Register(&protocol.ConnectMsg{Type: "connect", Info: info}, "s1")
	c.Check(err, Equals, broker.ErrUnexpectedValue)
	info["device"] = "m"
	info["channel"] = -1
	_, err = b.Register(&protocol.ConnectMsg{Type: "connect", Info: info}, "s2")
	c.Check(err, Equals, broker.ErrUnexpectedValue)
}

func (s *CommonBrokerSuite) TestRegistrationFeedPending(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M"}`)
	muchLater := time.Now().Add(10 * time.Minute)
	sto.AppendToChannel(store.SystemInternalChannelId, notification1, muchLater)
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	sess, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"}, "s1")
	c.Assert(err, IsNil)
	c.Check(len(sess.SessionChannel()), Equals, 2)
}

func (s *CommonBrokerSuite) TestRegistrationFeedPendingError(c *C) {
	sto := &testFailingStore{}
	b := s.MakeBroker(sto, testBrokerConfig, s.testlog)
	b.Start()
	defer b.Stop()
	_, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"}, "s1")
	c.Assert(err, IsNil)
	// but
	c.Check(s.testlog.Captured(), Matches, "ERROR unsuccessful, get channel snapshot for 0 \\(cachedOk=true\\): get channel snapshot fail\n")
}

func clearOfPending(c *C, sess broker.BrokerSession) {
	c.Assert(len(sess.SessionChannel()) >= 1, Equals, true)
	<-sess.SessionChannel()
}

func (s *CommonBrokerSuite) TestRegistrationLastWins(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	sess1, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"}, "s1")
	c.Assert(err, IsNil)
	clearOfPending(c, sess1)
	sess2, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"}, "s2")
	c.Assert(err, IsNil)
	// previous session got signaled by sending nil on its channel
	var sentinel broker.Exchange
	got := false
	select {
	case sentinel = <-sess1.SessionChannel():
		got = true
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to get sentinel")
	}
	c.Check(got, Equals, true)
	c.Check(sentinel, IsNil)
	c.Assert(s.RevealSession(b, "dev-1"), Equals, sess2)
	b.Unregister(sess1)
	// just to make sure the unregister was processed
	_, err = b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: ""}, "s3")
	c.Assert(err, IsNil)
	c.Check(s.RevealSession(b, "dev-1"), Equals, sess2)
}

func (s *CommonBrokerSuite) TestBroadcast(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M"}`)
	decoded1 := map[string]interface{}{"m": "M"}
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	sess1, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"}, "s1")
	c.Assert(err, IsNil)
	clearOfPending(c, sess1)
	sess2, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-2"}, "s2")
	c.Assert(err, IsNil)
	clearOfPending(c, sess2)
	// add notification to channel *after* the registrations
	muchLater := time.Now().Add(10 * time.Minute)
	sto.AppendToChannel(store.SystemInternalChannelId, notification1, muchLater)
	b.Broadcast(store.SystemInternalChannelId)
	select {
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to get broadcast exchange")
	case exchg1 := <-sess1.SessionChannel():
		c.Check(s.RevealBroadcastExchange(exchg1), DeepEquals, &broker.BroadcastExchange{
			ChanId:        store.SystemInternalChannelId,
			TopLevel:      1,
			Notifications: help.Ns(notification1),
			Decoded:       []map[string]interface{}{decoded1},
		})
	}
	select {
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to get broadcast exchange")
	case exchg2 := <-sess2.SessionChannel():
		c.Check(s.RevealBroadcastExchange(exchg2), DeepEquals, &broker.BroadcastExchange{
			ChanId:        store.SystemInternalChannelId,
			TopLevel:      1,
			Notifications: help.Ns(notification1),
			Decoded:       []map[string]interface{}{decoded1},
		})
	}
}

type testFailingStore struct {
	store.InMemoryPendingStore
	countdownToFail int
}

func (sto *testFailingStore) GetChannelSnapshot(chanId store.InternalChannelId) (int64, []protocol.Notification, error) {
	if sto.countdownToFail == 0 {
		return 0, nil, errors.New("get channel snapshot fail")
	}
	sto.countdownToFail--
	return 0, nil, nil
}

func (sto *testFailingStore) DropByMsgId(chanId store.InternalChannelId, targets []protocol.Notification) error {
	return errors.New("drop fail")
}

func (s *CommonBrokerSuite) TestBroadcastFail(c *C) {
	logged := make(chan bool, 1)
	s.testlog.SetLogEventCb(func(string) {
		logged <- true
	})
	sto := &testFailingStore{countdownToFail: 1}
	b := s.MakeBroker(sto, testBrokerConfig, s.testlog)
	b.Start()
	defer b.Stop()
	sess, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev-1"}, "s1")
	c.Assert(err, IsNil)
	clearOfPending(c, sess)
	b.Broadcast(store.SystemInternalChannelId)
	select {
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to log error")
	case <-logged:
	}
	c.Check(s.testlog.Captured(), Matches, "ERROR.*: get channel snapshot fail\n")
}

func (s *CommonBrokerSuite) TestUnicast(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M1"}`)
	notification2 := json.RawMessage(`{"m": "M2"}`)
	chanId1 := store.UnicastInternalChannelId("dev1", "dev1")
	chanId2 := store.UnicastInternalChannelId("dev2", "dev2")
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	sess1, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev1"}, "s1")
	c.Assert(err, IsNil)
	clearOfPending(c, sess1)
	sess2, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev2"}, "s2")
	c.Assert(err, IsNil)
	clearOfPending(c, sess2)
	// add notification to channel *after* the registrations
	muchLater := time.Now().Add(10 * time.Minute)
	sto.AppendToUnicastChannel(chanId1, "app1", notification1, "msg1", muchLater)
	sto.AppendToUnicastChannel(chanId2, "app2", notification2, "msg2", muchLater)
	b.Unicast(chanId2, chanId1)
	select {
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to get unicast exchange")
	case exchg1 := <-sess1.SessionChannel():
		u1 := s.RevealUnicastExchange(exchg1)
		c.Check(u1.ChanId, Equals, chanId1)
	}
	select {
	case <-time.After(5 * time.Second):
		c.Fatal("taking too long to get unicast exchange")
	case exchg2 := <-sess2.SessionChannel():
		u2 := s.RevealUnicastExchange(exchg2)
		c.Check(u2.ChanId, Equals, chanId2)
	}
}

func (s *CommonBrokerSuite) TestGetAndDrop(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M1"}`)
	chanId1 := store.UnicastInternalChannelId("dev3", "dev3")
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	sess1, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev3"}, "s1")
	c.Assert(err, IsNil)
	muchLater := time.Now().Add(10 * time.Minute)
	sto.AppendToUnicastChannel(chanId1, "app1", notification1, "msg1", muchLater)
	_, expected, err := sto.GetChannelSnapshot(chanId1)
	c.Assert(err, IsNil)
	_, notifs, err := sess1.Get(chanId1, false)
	c.Check(notifs, HasLen, 1)
	c.Check(notifs, DeepEquals, expected)
	err = sess1.DropByMsgId(chanId1, notifs)
	c.Assert(err, IsNil)
	_, notifs, err = sess1.Get(chanId1, true)
	c.Check(notifs, HasLen, 0)
	_, expected, err = sto.GetChannelSnapshot(chanId1)
	c.Assert(err, IsNil)
	c.Check(expected, HasLen, 0)

}

func (s *CommonBrokerSuite) TestGetAndDropErrors(c *C) {
	chanId1 := store.UnicastInternalChannelId("dev3", "dev3")
	sto := &testFailingStore{countdownToFail: 1}
	b := s.MakeBroker(sto, testBrokerConfig, s.testlog)
	b.Start()
	defer b.Stop()
	sess1, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev3"}, "s1")
	c.Assert(err, IsNil)
	_, _, err = sess1.Get(chanId1, false)
	c.Assert(err, ErrorMatches, "get channel snapshot fail")
	c.Check(s.testlog.Captured(), Matches, "ERROR unsuccessful, get channel snapshot for Udev3:dev3 \\(cachedOk=false\\): get channel snapshot fail\n")
	s.testlog.ResetCapture()

	err = sess1.DropByMsgId(chanId1, nil)
	c.Assert(err, ErrorMatches, "drop fail")
	c.Check(s.testlog.Captured(), Matches, "ERROR unsuccessful, drop from channel Udev3:dev3: drop fail\n")
}

func (s *CommonBrokerSuite) TestSessionFeed(c *C) {
	sto := store.NewInMemoryPendingStore()
	b := s.MakeBroker(sto, testBrokerConfig, nil)
	b.Start()
	defer b.Stop()
	sess1, err := b.Register(&protocol.ConnectMsg{Type: "connect", DeviceId: "dev3"}, "s1")
	c.Assert(err, IsNil)
	clearOfPending(c, sess1)
	bcast := &broker.BroadcastExchange{ChanId: store.SystemInternalChannelId, TopLevel: 99}
	sess1.Feed(bcast)
	c.Check(s.RevealBroadcastExchange(<-sess1.SessionChannel()), DeepEquals, bcast)

	ucast := &broker.UnicastExchange{ChanId: store.UnicastInternalChannelId("dev21", "dev21"), CachedOk: true}
	sess1.Feed(ucast)
	c.Check(s.RevealUnicastExchange(<-sess1.SessionChannel()), DeepEquals, ucast)
}
