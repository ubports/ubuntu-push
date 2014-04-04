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
	"fmt"
	"strings"
	stdtesting "testing"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/server/broker"
	"launchpad.net/ubuntu-push/server/broker/testing"
	"launchpad.net/ubuntu-push/server/store"
)

func TestBroker(t *stdtesting.T) { TestingT(t) }

type exchangesSuite struct{}

var _ = Suite(&exchangesSuite{})

func (s *exchangesSuite) TestBroadcastExchangeInit(c *C) {
	exchg := &broker.BroadcastExchange{
		ChanId:   store.SystemInternalChannelId,
		TopLevel: 3,
		NotificationPayloads: []json.RawMessage{
			json.RawMessage(`{"a":"x"}`),
			json.RawMessage(`[]`),
			json.RawMessage(`{"a":"y"}`),
		},
	}
	exchg.Init()
	c.Check(exchg.Decoded, DeepEquals, []map[string]interface{}{
		map[string]interface{}{"a": "x"},
		nil,
		map[string]interface{}{"a": "y"},
	})
}

func (s *exchangesSuite) TestBroadcastExchange(c *C) {
	sess := &testing.TestBrokerSession{
		LevelsMap:    broker.LevelsMap(map[store.InternalChannelId]int64{}),
		Model:        "m1",
		ImageChannel: "img1",
	}
	exchg := &broker.BroadcastExchange{
		ChanId:   store.SystemInternalChannelId,
		TopLevel: 3,
		NotificationPayloads: []json.RawMessage{
			json.RawMessage(`{"img1/m1":100}`),
			json.RawMessage(`{"img2/m2":200}`),
		},
	}
	exchg.Init()
	outMsg, inMsg, err := exchg.Prepare(sess)
	c.Assert(err, IsNil)
	// check
	marshalled, err := json.Marshal(outMsg)
	c.Assert(err, IsNil)
	c.Check(string(marshalled), Equals, `{"T":"broadcast","ChanId":"0","TopLevel":3,"Payloads":[{"img1/m1":100}]}`)
	err = json.Unmarshal([]byte(`{"T":"ack"}`), inMsg)
	c.Assert(err, IsNil)
	err = exchg.Acked(sess, true)
	c.Assert(err, IsNil)
	c.Check(sess.LevelsMap[store.SystemInternalChannelId], Equals, int64(3))
}

func (s *exchangesSuite) TestBroadcastExchangeEmpty(c *C) {
	sess := &testing.TestBrokerSession{
		LevelsMap:    broker.LevelsMap(map[store.InternalChannelId]int64{}),
		Model:        "m1",
		ImageChannel: "img1",
	}
	exchg := &broker.BroadcastExchange{
		ChanId:               store.SystemInternalChannelId,
		TopLevel:             3,
		NotificationPayloads: []json.RawMessage{},
	}
	exchg.Init()
	outMsg, inMsg, err := exchg.Prepare(sess)
	c.Assert(err, Equals, broker.ErrNop)
	c.Check(outMsg, IsNil)
	c.Check(inMsg, IsNil)
}

func (s *exchangesSuite) TestBroadcastExchangeEmptyButAhead(c *C) {
	sess := &testing.TestBrokerSession{
		LevelsMap: broker.LevelsMap(map[store.InternalChannelId]int64{
			store.SystemInternalChannelId: 10,
		}),
		Model:        "m1",
		ImageChannel: "img1",
	}
	exchg := &broker.BroadcastExchange{
		ChanId:               store.SystemInternalChannelId,
		TopLevel:             3,
		NotificationPayloads: []json.RawMessage{},
	}
	exchg.Init()
	outMsg, inMsg, err := exchg.Prepare(sess)
	c.Assert(err, IsNil)
	c.Check(outMsg, NotNil)
	c.Check(inMsg, NotNil)
}

func (s *exchangesSuite) TestBroadcastExchangeReuseVsSplit(c *C) {
	sess := &testing.TestBrokerSession{
		LevelsMap:    broker.LevelsMap(map[store.InternalChannelId]int64{}),
		Model:        "m1",
		ImageChannel: "img1",
	}
	payloadFmt := fmt.Sprintf(`{"img1/m1":%%d,"bloat":"%s"}`, strings.Repeat("x", 1024*2))
	needsSplitting := make([]json.RawMessage, 32)
	for i := 0; i < 32; i++ {
		needsSplitting[i] = json.RawMessage(fmt.Sprintf(payloadFmt, i))
	}

	topLevel := int64(len(needsSplitting))
	exchg := &broker.BroadcastExchange{
		ChanId:               store.SystemInternalChannelId,
		TopLevel:             topLevel,
		NotificationPayloads: needsSplitting,
	}
	exchg.Init()
	outMsg, _, err := exchg.Prepare(sess)
	c.Assert(err, IsNil)
	parts := 0
	for {
		done := outMsg.Split()
		parts++
		if done {
			break
		}
	}
	c.Assert(parts, Equals, 2)
	exchg = &broker.BroadcastExchange{
		ChanId:   store.SystemInternalChannelId,
		TopLevel: topLevel + 2,
		NotificationPayloads: []json.RawMessage{
			json.RawMessage(`{"img1/m1":"x"}`),
			json.RawMessage(`{"img1/m1":"y"}`),
		},
	}
	exchg.Init()
	outMsg, _, err = exchg.Prepare(sess)
	c.Assert(err, IsNil)
	done := outMsg.Split() // shouldn't panic
	c.Check(done, Equals, true)
}

func (s *exchangesSuite) TestBroadcastExchangeAckMismatch(c *C) {
	sess := &testing.TestBrokerSession{
		LevelsMap:    broker.LevelsMap(map[store.InternalChannelId]int64{}),
		Model:        "m1",
		ImageChannel: "img2",
	}
	exchg := &broker.BroadcastExchange{
		ChanId:   store.SystemInternalChannelId,
		TopLevel: 3,
		NotificationPayloads: []json.RawMessage{
			json.RawMessage(`{"img2/m1":1}`),
		},
	}
	exchg.Init()
	outMsg, inMsg, err := exchg.Prepare(sess)
	c.Assert(err, IsNil)
	// check
	marshalled, err := json.Marshal(outMsg)
	c.Assert(err, IsNil)
	c.Check(string(marshalled), Equals, `{"T":"broadcast","ChanId":"0","TopLevel":3,"Payloads":[{"img2/m1":1}]}`)
	err = json.Unmarshal([]byte(`{}`), inMsg)
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
		Model:        "m1",
		ImageChannel: "img1",
	}
	exchg := &broker.BroadcastExchange{
		ChanId:   store.SystemInternalChannelId,
		TopLevel: 3,
		NotificationPayloads: []json.RawMessage{
			json.RawMessage(`{"img1/m1":100}`),
			json.RawMessage(`{"img1/m1":101}`),
		},
	}
	exchg.Init()
	outMsg, inMsg, err := exchg.Prepare(sess)
	c.Assert(err, IsNil)
	// check
	marshalled, err := json.Marshal(outMsg)
	c.Assert(err, IsNil)
	c.Check(string(marshalled), Equals, `{"T":"broadcast","ChanId":"0","TopLevel":3,"Payloads":[{"img1/m1":101}]}`)
	err = json.Unmarshal([]byte(`{"T":"ack"}`), inMsg)
	c.Assert(err, IsNil)
	err = exchg.Acked(sess, true)
	c.Assert(err, IsNil)
}

func (s *exchangesSuite) TestBroadcastExchangeChannelFilter(c *C) {
	sess := &testing.TestBrokerSession{
		LevelsMap:    broker.LevelsMap(map[store.InternalChannelId]int64{}),
		Model:        "m1",
		ImageChannel: "img1",
	}
	exchg := &broker.BroadcastExchange{
		ChanId:   store.SystemInternalChannelId,
		TopLevel: 5,
		NotificationPayloads: []json.RawMessage{
			json.RawMessage(`{"img1/m1":100}`),
			json.RawMessage(`{"img2/m2":200}`),
			json.RawMessage(`{"img1/m1":101}`),
		},
	}
	exchg.Init()
	outMsg, inMsg, err := exchg.Prepare(sess)
	c.Assert(err, IsNil)
	// check
	marshalled, err := json.Marshal(outMsg)
	c.Assert(err, IsNil)
	c.Check(string(marshalled), Equals, `{"T":"broadcast","ChanId":"0","TopLevel":5,"Payloads":[{"img1/m1":100},{"img1/m1":101}]}`)
	err = json.Unmarshal([]byte(`{"T":"ack"}`), inMsg)
	c.Assert(err, IsNil)
	err = exchg.Acked(sess, true)
	c.Assert(err, IsNil)
	c.Check(sess.LevelsMap[store.SystemInternalChannelId], Equals, int64(5))
}

func (s *exchangesSuite) TestConnBrokenExchange(c *C) {
	sess := &testing.TestBrokerSession{}
	cbe := &broker.ConnBrokenExchange{"REASON"}
	outMsg, inMsg, err := cbe.Prepare(sess)
	c.Assert(err, IsNil)
	c.Check(inMsg, IsNil) // no answer is expected
	// check
	marshalled, err := json.Marshal(outMsg)
	c.Assert(err, IsNil)
	c.Check(string(marshalled), Equals, `{"T":"connbroken","Reason":"REASON"}`)

	c.Check(func() { cbe.Acked(nil, true) }, PanicMatches, "Acked should not get invoked on ConnBrokenExchange")
}
