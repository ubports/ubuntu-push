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
	"fmt"

	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/store"
)

// Exchanges

// Scratch area for exchanges, sessions should hold one of these.
type ExchangesScratchArea struct {
	broadcastMsg protocol.BroadcastMsg
	ackMsg       protocol.AckMsg
}

// BroadcastExchange leads a session through delivering a BROADCAST.
// For simplicity it is fully public.
type BroadcastExchange struct {
	ChanId               store.InternalChannelId
	TopLevel             int64
	NotificationPayloads []json.RawMessage
	Decoded              []map[string]interface{}
}

// check interface already here
var _ Exchange = &BroadcastExchange{}

// Init ensures the BroadcastExchange is fully initialized for the sessions.
func (sbe *BroadcastExchange) Init() {
	decoded := make([]map[string]interface{}, len(sbe.NotificationPayloads))
	sbe.Decoded = decoded
	for i, p := range sbe.NotificationPayloads {
		err := json.Unmarshal(p, &decoded[i])
		if err != nil {
			decoded[i] = nil
		}
	}
}

func filterByLevel(clientLevel, topLevel int64, payloads []json.RawMessage) []json.RawMessage {
	c := int64(len(payloads))
	if c == 0 {
		return nil
	}
	delta := topLevel - clientLevel
	if delta < 0 { // means too ahead, send the last pending
		delta = 1
	}
	if delta < c {
		return payloads[c-delta:]
	} else {
		return payloads
	}
}

func channelFilter(tag string, chanId store.InternalChannelId, payloads []json.RawMessage, decoded []map[string]interface{}) []json.RawMessage {
	if len(payloads) != 0 && chanId == store.SystemInternalChannelId {
		decoded := decoded[len(decoded)-len(payloads):]
		filtered := make([]json.RawMessage, 0)
		for i, decoded1 := range decoded {
			if _, ok := decoded1[tag]; ok {
				filtered = append(filtered, payloads[i])
			}
		}
		payloads = filtered
	}
	return payloads
}

// Prepare session for a BROADCAST.
func (sbe *BroadcastExchange) Prepare(sess BrokerSession) (outMessage protocol.SplittableMsg, inMessage interface{}, err error) {
	clientLevel := sess.Levels()[sbe.ChanId]
	payloads := filterByLevel(clientLevel, sbe.TopLevel, sbe.NotificationPayloads)
	tag := fmt.Sprintf("%s/%s", sess.DeviceImageChannel(), sess.DeviceImageModel())
	payloads = channelFilter(tag, sbe.ChanId, payloads, sbe.Decoded)
	if len(payloads) == 0 && sbe.TopLevel >= clientLevel {
		// empty and don't need to force resync => do nothing
		return nil, nil, ErrNop
	}

	scratchArea := sess.ExchangeScratchArea()
	scratchArea.broadcastMsg.Reset()
	scratchArea.broadcastMsg.Type = "broadcast"
	// xxx need an AppId as well, later
	scratchArea.broadcastMsg.ChanId = store.InternalChannelIdToHex(sbe.ChanId)
	scratchArea.broadcastMsg.TopLevel = sbe.TopLevel
	scratchArea.broadcastMsg.Payloads = payloads
	return &scratchArea.broadcastMsg, &scratchArea.ackMsg, nil
}

// Acked deals with an ACK for a BROADCAST.
func (sbe *BroadcastExchange) Acked(sess BrokerSession, done bool) error {
	scratchArea := sess.ExchangeScratchArea()
	if scratchArea.ackMsg.Type != "ack" {
		return &ErrAbort{"expected ACK message"}
	}
	// update levels
	sess.Levels()[sbe.ChanId] = sbe.TopLevel
	return nil
}
