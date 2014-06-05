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
	"time"

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

type BaseExchange struct {
	Timestamp time.Time
}

// BroadcastExchange leads a session through delivering a BROADCAST.
// For simplicity it is fully public.
type BroadcastExchange struct {
	ChanId        store.InternalChannelId
	TopLevel      int64
	Notifications []protocol.Notification
	Decoded       []map[string]interface{}
	BaseExchange
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
	ChanId   store.InternalChannelId
	CachedOk bool
	BaseExchange
}

// check interface already here
var _ Exchange = (*UnicastExchange)(nil)

// Prepare session for a NOTIFICATIONS.
func (sue *UnicastExchange) Prepare(sess BrokerSession) (outMessage protocol.SplittableMsg, inMessage interface{}, err error) {
	_, notifs, err := sess.Get(sue.ChanId, sue.CachedOk)
	if err != nil {
		return nil, nil, err
	}
	if len(notifs) == 0 {
		return nil, nil, ErrNop
	}
	scratchArea := sess.ExchangeScratchArea()
	scratchArea.notificationsMsg.Reset()
	scratchArea.notificationsMsg.Notifications = notifs
	return &scratchArea.notificationsMsg, &scratchArea.ackMsg, nil
}

// Acked deals with an ACK for a NOTIFICATIONS.
func (sue *UnicastExchange) Acked(sess BrokerSession, done bool) error {
	scratchArea := sess.ExchangeScratchArea()
	if scratchArea.ackMsg.Type != "ack" {
		return &ErrAbort{"expected ACK message"}
	}
	err := sess.DropByMsgId(sue.ChanId, scratchArea.notificationsMsg.Notifications)
	if err != nil {
		return err
	}
	return nil
}

// FeedPending feeds exchanges covering pending notifications into the session.
func FeedPending(sess BrokerSession) error {
	// find relevant channels, for now only system
	channels := []store.InternalChannelId{store.SystemInternalChannelId}
	for _, chanId := range channels {
		topLevel, notifications, err := sess.Get(chanId, true)
		if err != nil {
			// next broadcast will try again
			continue
		}
		clientLevel := sess.Levels()[chanId]
		if clientLevel != topLevel {
			broadcastExchg := &BroadcastExchange{
				ChanId:        chanId,
				TopLevel:      topLevel,
				Notifications: notifications,
			}
			broadcastExchg.Init()
			sess.Feed(broadcastExchg)
		}
	}
	sess.Feed(&UnicastExchange{ChanId: sess.InternalChannelId(), CachedOk: true})
	return nil
}
