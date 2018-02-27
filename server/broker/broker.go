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

// Package broker handles session registrations and delivery of messages
// through sessions.
package broker

import (
	"errors"
	"fmt"

	"github.com/ubports/ubuntu-push/protocol"
	"github.com/ubports/ubuntu-push/server/store"
)

type SessionTracker interface {
	// SessionId
	SessionId() string
}

// Broker is responsible for registring sessions and delivering messages
// through them.
type Broker interface {
	// Register the session.
	Register(connMsg *protocol.ConnectMsg, track SessionTracker) (BrokerSession, error)
	// Unregister the session.
	Unregister(BrokerSession)
}

// BrokerSending is the notification sending facet of the broker.
type BrokerSending interface {
	// Broadcast channel.
	Broadcast(chanId store.InternalChannelId)
	// Unicast over channels.
	Unicast(chanIds ...store.InternalChannelId)
}

// Exchange leads the session through performing an exchange, typically delivery.
type Exchange interface {
	Prepare(sess BrokerSession) (outMessage protocol.SplittableMsg, inMessage interface{}, err error)
	Acked(sess BrokerSession, done bool) error
}

// ErrNop returned by Prepare means nothing to do/send.
var ErrNop = errors.New("nothing to send")

// LevelsMap is the type for holding channel levels for session.
type LevelsMap map[store.InternalChannelId]int64

// GetInfoString helps retrieveng a string out of a protocol.ConnectMsg.Info.
func GetInfoString(msg *protocol.ConnectMsg, name, defaultVal string) (string, error) {
	v, ok := msg.Info[name]
	if !ok {
		return defaultVal, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", ErrUnexpectedValue
	}
	return s, nil
}

// GetInfoInt helps retrieving an integer out of a protocol.ConnectMsg.Info.
func GetInfoInt(msg *protocol.ConnectMsg, name string, defaultVal int) (int, error) {
	v, ok := msg.Info[name]
	if !ok {
		return defaultVal, nil
	}
	n, ok := v.(float64)
	if !ok {
		return -1, ErrUnexpectedValue
	}
	return int(n), nil
}

// BrokerSession holds broker session state.
type BrokerSession interface {
	// SessionChannel returns the session control channel
	// on which the session gets exchanges to perform.
	SessionChannel() <-chan Exchange
	// DeviceIdentifier returns the device id string.
	DeviceIdentifier() string
	// DeviceImageModel returns the device model.
	DeviceImageModel() string
	// DeviceImageChannel returns the device system image channel.
	DeviceImageChannel() string
	// Levels returns the current channel levels for the session
	Levels() LevelsMap
	// ExchangeScratchArea returns the scratch area for exchanges.
	ExchangeScratchArea() *ExchangesScratchArea
	// Get gets the content of the channel with chanId.
	Get(chanId store.InternalChannelId, cachedOk bool) (int64, []protocol.Notification, error)
	// DropByMsgId drops notifications from the channel chanId by message id.
	DropByMsgId(chanId store.InternalChannelId, targets []protocol.Notification) error
	// Feed feeds exchange into the session.
	Feed(Exchange)
	// InternalChannelId() returns the channel id corresponding to the session.
	InternalChannelId() store.InternalChannelId
}

// Session aborted error.
type ErrAbort struct {
	Reason string
}

func (ea *ErrAbort) Error() string {
	return fmt.Sprintf("session aborted (%s)", ea.Reason)
}

// Unexpect value in message
var ErrUnexpectedValue = &ErrAbort{"unexpected value in message"}

// BrokerConfig gives access to the typical broker configuration.
type BrokerConfig interface {
	// SessionQueueSize gives the session queue size.
	SessionQueueSize() uint
	// BrokerQueueSize gives the internal broker queue size.
	BrokerQueueSize() uint
}
