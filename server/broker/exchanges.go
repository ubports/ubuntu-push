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
	broadcastMsg     protocol.BroadcastMsg
	notificationsMsg protocol.NotificationsMsg
	ackMsg           protocol.AckMsg
}

// BroadcastExchange leads a session through delivering a BROADCAST.
// For simplicity it is fully public.
type BroadcastExchange struct {
	ChanId        store.InternalChannelId
	TopLevel      int64
	Notifications []protocol.Notification
	Decoded       []map[string]interface{}
}

// check interface already here
var _ Exchange = (*BroadcastExchange)(nil)

// Init ensures the BroadcastExchange is fully initialized for the sessions.
func (sbe *BroadcastExchange) Init() {
	decoded := make([]map[string]interface{}, len(sbe.Notifications))
	sbe.Decoded = decoded
	for i, notif := range sbe.Notifications {
		err := json.Unmarshal(notif.Payload, &decoded[i])
		if err != nil {
			decoded[i] = nil
		}
	}
}

func filterByLevel(clientLevel, topLevel int64, notifs []protocol.Notification) []protocol.Notification {
	c := int64(len(notifs))
	if c == 0 {
		return nil
	}
	delta := topLevel - clientLevel
	if delta < 0 { // means too ahead, send the last pending
		delta = 1
	}
	if delta < c {
		return notifs[c-delta:]
	} else {
		return notifs
	}
}

func channelFilter(tag string, chanId store.InternalChannelId, notifs []protocol.Notification, decoded []map[string]interface{}) []json.RawMessage {
	if len(notifs) != 0 && chanId == store.SystemInternalChannelId {
		decoded := decoded[len(decoded)-len(notifs):]
		filtered := make([]json.RawMessage, 0)
		for i, decoded1 := range decoded {
			if _, ok := decoded1[tag]; ok {
				filtered = append(filtered, notifs[i].Payload)
			}
		}
		return filtered
	}
	return protocol.ExtractPayloads(notifs)
}

// Prepare session for a BROADCAST.
func (sbe *BroadcastExchange) Prepare(sess BrokerSession) (outMessage protocol.SplittableMsg, inMessage interface{}, err error) {
	clientLevel := sess.Levels()[sbe.ChanId]
	notifs := filterByLevel(clientLevel, sbe.TopLevel, sbe.Notifications)
	tag := fmt.Sprintf("%s/%s", sess.DeviceImageChannel(), sess.DeviceImageModel())
	payloads := channelFilter(tag, sbe.ChanId, notifs, sbe.Decoded)
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

// ConnMetaExchange allows to send a CONNBROKEN or CONNWARN message.
type ConnMetaExchange struct {
	Msg protocol.OnewayMsg
}

// check interface already here
var _ Exchange = (*ConnMetaExchange)(nil)

// Prepare session for a CONNBROKEN/WARN.
func (cbe *ConnMetaExchange) Prepare(sess BrokerSession) (outMessage protocol.SplittableMsg, inMessage interface{}, err error) {
	return cbe.Msg, nil, nil
}

// CONNBROKEN/WARN aren't acked.
func (cbe *ConnMetaExchange) Acked(sess BrokerSession, done bool) error {
	panic("Acked should not get invoked on ConnMetaExchange")
}

// UnicastExchange leads a session through delivering a NOTIFICATIONS message.
// For simplicity it is fully public.
type UnicastExchange struct {
	// Get retrieves the notifications to send
	Get func() ([]protocol.Notification, error)
	// DropByMsgId drops the sent notifications
	DropByMsgId func([]protocol.Notification) error
}

// check interface already here
var _ Exchange = (*UnicastExchange)(nil)

// Prepare session for a NOTIFICATIONS.
func (sue *UnicastExchange) Prepare(sess BrokerSession) (outMessage protocol.SplittableMsg, inMessage interface{}, err error) {
	notifs, err := sue.Get()
	if err != nil {
		return nil, nil, err
	}
	scratchArea := sess.ExchangeScratchArea()
	scratchArea.notificationsMsg.Reset()
	scratchArea.notificationsMsg.Type = "notifications"
	scratchArea.notificationsMsg.Notifications = notifs
	return &scratchArea.notificationsMsg, &scratchArea.ackMsg, nil
}

// Acked deals with an ACK for a NOTIFICATIONS.
func (sue *UnicastExchange) Acked(sess BrokerSession, done bool) error {
	scratchArea := sess.ExchangeScratchArea()
	if scratchArea.ackMsg.Type != "ack" {
		return &ErrAbort{"expected ACK message"}
	}
	err := sue.DropByMsgId(scratchArea.notificationsMsg.Notifications)
	if err != nil {
		return err
	}
	return nil
}
