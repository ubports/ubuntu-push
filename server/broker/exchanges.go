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
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/store"
	// "log"
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
}

func filterByLevel(clientLevel, topLevel int64, payloads []json.RawMessage) []json.RawMessage {
	c := int64(len(payloads))
	delta := topLevel - clientLevel
	if delta < c {
		return payloads[c-delta:]
	} else {
		return payloads
	}
}

// Prepare session for a BROADCAST.
func (sbe *BroadcastExchange) Prepare(sess BrokerSession) (outMessage protocol.SplittableMsg, inMessage interface{}, err error) {
	scratchArea := sess.ExchangeScratchArea()
	scratchArea.broadcastMsg.Type = "broadcast"
	clientLevel := sess.Levels()[sbe.ChanId]
	payloads := filterByLevel(clientLevel, sbe.TopLevel, sbe.NotificationPayloads)
	// xxx need an AppId as well, later
	scratchArea.broadcastMsg.ChanId = store.InternalChannelIdToHex(sbe.ChanId)
	scratchArea.broadcastMsg.TopLevel = sbe.TopLevel
	scratchArea.broadcastMsg.Payloads = payloads
	return &scratchArea.broadcastMsg, &scratchArea.ackMsg, nil
}

// Acked deals with an ACK for a BROADCAST.
func (sbe *BroadcastExchange) Acked(sess BrokerSession) error {
	scratchArea := sess.ExchangeScratchArea()
	if scratchArea.ackMsg.Type != "ack" {
		return &ErrAbort{"expected ACK message"}
	}
	// update levels
	sess.Levels()[sbe.ChanId] = sbe.TopLevel
	return nil
}
