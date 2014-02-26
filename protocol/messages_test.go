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

package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	. "launchpad.net/gocheck"
)

type messagesSuite struct{}

var _ = Suite(&messagesSuite{})

func (s *messagesSuite) TestSplitBroadcastMsgNop(c *C) {
	b := &BroadcastMsg{
		Type:     "broadcast",
		AppId:    "APP",
		ChanId:   "0",
		TopLevel: 2,
		Payloads: []json.RawMessage{json.RawMessage(`{b:1}`), json.RawMessage(`{b:2}`)},
	}
	done := b.Split()
	c.Check(done, Equals, true)
	c.Check(b.TopLevel, Equals, int64(2))
	c.Check(cap(b.Payloads), Equals, 2)
	c.Check(len(b.Payloads), Equals, 2)
}

var payloadFmt = fmt.Sprintf(`{"b":%%d,"bloat":"%s"}`, strings.Repeat("x", 1024*2))

func manyParts(c int) []json.RawMessage {
	payloads := make([]json.RawMessage, 0, 1)
	for i := 0; i < c; i++ {
		payloads = append(payloads, json.RawMessage(fmt.Sprintf(payloadFmt, i)))
	}
	return payloads
}

func (s *messagesSuite) TestSplitBroadcastMsgManyParts(c *C) {
	payloads := manyParts(33)
	n := len(payloads)
	// more interesting this way
	c.Assert(cap(payloads), Not(Equals), n)
	b := &BroadcastMsg{
		Type:     "broadcast",
		AppId:    "APP",
		ChanId:   "0",
		TopLevel: 500,
		Payloads: payloads,
	}
	done := b.Split()
	c.Assert(done, Equals, false)
	n1 := len(b.Payloads)
	c.Check(b.TopLevel, Equals, int64(500-n+n1))
	buf, err := json.Marshal(b)
	c.Assert(err, IsNil)
	c.Assert(len(buf) <= 65535, Equals, true)
	c.Check(len(buf)+len(payloads[n1]) > maxPayloadSize, Equals, true)
	done = b.Split()
	c.Assert(done, Equals, true)
	n2 := len(b.Payloads)
	c.Check(b.TopLevel, Equals, int64(500))
	c.Check(n1+n2, Equals, n)

	payloads = manyParts(61)
	n = len(payloads)
	b = &BroadcastMsg{
		Type:     "broadcast",
		AppId:    "APP",
		ChanId:   "0",
		TopLevel: int64(n),
		Payloads: payloads,
	}
	done = b.Split()
	c.Assert(done, Equals, false)
	n1 = len(b.Payloads)
	done = b.Split()
	c.Assert(done, Equals, false)
	n2 = len(b.Payloads)
	done = b.Split()
	c.Assert(done, Equals, true)
	n3 := len(b.Payloads)
	c.Check(b.TopLevel, Equals, int64(n))
	c.Check(n1+n2+n3, Equals, n)
	// reset
	b.Reset()
	c.Check(b.splitting, Equals, 0)
}
