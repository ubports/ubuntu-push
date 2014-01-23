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
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/server/store"
	// "log"
)

type exchangesSuite struct{}

var _ = Suite(&exchangesSuite{})

type testBrokerSession struct {
	deviceId     string
	exchanges    chan Exchange
	levels       LevelsMap
	exchgScratch ExchangesScratchArea
}

func (tbs *testBrokerSession) SessionChannel() <-chan Exchange {
	return nil
}

func (tbs *testBrokerSession) DeviceId() string {
	return ""
}

func (tbs *testBrokerSession) Levels() LevelsMap {
	return tbs.levels
}

func (tbs *testBrokerSession) ExchangeScratchArea() *ExchangesScratchArea {
	return &tbs.exchgScratch
}

func (s *exchangesSuite) TestBroadcastExchange(c *C) {
	sess := &testBrokerSession{
		levels: map[store.InternalChannelId]int64{},
	}
	exchg := &BroadcastExchange{
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
	err = exchg.Acked(sess)
	c.Assert(err, IsNil)
	c.Check(sess.levels[store.SystemInternalChannelId], Equals, int64(3))
}

func (s *exchangesSuite) TestBroadcastExchangeAckMismatch(c *C) {
	sess := &testBrokerSession{
		levels: map[store.InternalChannelId]int64{},
	}
	exchg := &BroadcastExchange{
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
	err = exchg.Acked(sess)
	c.Assert(err, Not(IsNil))
	c.Check(sess.levels[store.SystemInternalChannelId], Equals, int64(0))
}

func (s *exchangesSuite) TestFilterByLevel(c *C) {
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

func (s *exchangesSuite) TestBroadcastExchangeFilterByLevel(c *C) {
	sess := &testBrokerSession{
		levels: map[store.InternalChannelId]int64{
			store.SystemInternalChannelId: 2,
		},
	}
	exchg := &BroadcastExchange{
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
	err = exchg.Acked(sess)
	c.Assert(err, IsNil)
}
