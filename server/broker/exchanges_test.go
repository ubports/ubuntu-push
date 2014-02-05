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

package broker_test // use a package test to avoid cyclic imports

import (
	"encoding/json"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/server/broker"
	"launchpad.net/ubuntu-push/server/broker/testing"
	"launchpad.net/ubuntu-push/server/store"
	// "log"
	gotesting "testing"
)

func TestBroker(t *gotesting.T) { TestingT(t) }

type exchangesSuite struct{}

var _ = Suite(&exchangesSuite{})

func (s *exchangesSuite) TestBroadcastExchange(c *C) {
	sess := &testing.TestBrokerSession{
		LevelsMap: broker.LevelsMap(map[store.InternalChannelId]int64{}),
	}
	exchg := &broker.BroadcastExchange{
		ChanId:   store.SystemInternalChannelId,
		TopLevel: 3,
		NotificationPayloads: []json.RawMessage{
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
	err = exchg.Acked(sess, true)
	c.Assert(err, IsNil)
	c.Check(sess.LevelsMap[store.SystemInternalChannelId], Equals, int64(3))
}

func (s *exchangesSuite) TestBroadcastExchangeAckMismatch(c *C) {
	sess := &testing.TestBrokerSession{
		LevelsMap: broker.LevelsMap(map[store.InternalChannelId]int64{}),
	}
	exchg := &broker.BroadcastExchange{
		ChanId:   store.SystemInternalChannelId,
		TopLevel: 3,
		NotificationPayloads: []json.RawMessage{
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
	err = exchg.Acked(sess, true)
	c.Assert(err, Not(IsNil))
	c.Check(sess.LevelsMap[store.SystemInternalChannelId], Equals, int64(0))
}

func (s *exchangesSuite) TestBroadcastExchangeFilterByLevel(c *C) {
	sess := &testing.TestBrokerSession{
		LevelsMap: broker.LevelsMap(map[store.InternalChannelId]int64{
			store.SystemInternalChannelId: 2,
		}),
	}
	exchg := &broker.BroadcastExchange{
		ChanId:   store.SystemInternalChannelId,
		TopLevel: 3,
		NotificationPayloads: []json.RawMessage{
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
	err = exchg.Acked(sess, true)
	c.Assert(err, IsNil)
}
