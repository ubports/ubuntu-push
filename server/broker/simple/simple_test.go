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

package simple

import (
	"encoding/json"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/server/broker"
	"launchpad.net/ubuntu-push/server/store"
	// "log"
	"testing"
)

func TestSimple(t *testing.T) { TestingT(t) }

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

func (s *simpleSuite) TestFeedPending(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M"}`)
	sto.AppendToChannel(store.SystemInternalChannelId, notification1)
	b := NewSimpleBroker(sto, &testBrokerConfig{}, nil)
	sess := &simpleBrokerSession{
		exchanges: make(chan broker.Exchange, 1),
	}
	b.feedPending(sess)
	c.Assert(len(sess.exchanges), Equals, 1)
	exchg1 := <-sess.exchanges
	c.Check(exchg1, DeepEquals, &broker.BroadcastExchange{
		ChanId:               store.SystemInternalChannelId,
		TopLevel:             1,
		NotificationPayloads: []json.RawMessage{notification1},
	})
}

func (s *simpleSuite) TestFeedPendingNop(c *C) {
	sto := store.NewInMemoryPendingStore()
	notification1 := json.RawMessage(`{"m": "M"}`)
	sto.AppendToChannel(store.SystemInternalChannelId, notification1)
	b := NewSimpleBroker(sto, &testBrokerConfig{}, nil)
	sess := &simpleBrokerSession{
		exchanges: make(chan broker.Exchange, 1),
		levels: map[store.InternalChannelId]int64{
			store.SystemInternalChannelId: 1,
		},
	}
	b.feedPending(sess)
	c.Assert(len(sess.exchanges), Equals, 0)
}
