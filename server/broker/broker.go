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
	"fmt"

	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/store"
)

// Broker is responsible for registring sessions and delivering messages through them.
type Broker interface {
	// Register the session.
	Register(*protocol.ConnectMsg) (BrokerSession, error)
	// Unregister the session.
	Unregister(BrokerSession)
}

// BrokerSending is the notification sending facet of the broker.
type BrokerSending interface {
	// Broadcast channel.
	Broadcast(chanId store.InternalChannelId)
}

// Exchange leads the session through performing an exchange, typically delivery.
type Exchange interface {
	Prepare(sess BrokerSession) (outMessage protocol.SplittableMsg, inMessage interface{}, err error)
	Acked(sess BrokerSession, done bool) error
}

// LevelsMap is the type for holding channel levels for session.
type LevelsMap map[store.InternalChannelId]int64

// BrokerSession holds broker session state.
type BrokerSession interface {
	// SessionChannel returns the session control channel
	// on which the session gets exchanges to perform.
	SessionChannel() <-chan Exchange
	// DeviceIdentifier returns the device id string.
	DeviceIdentifier() string
	// Levels returns the current channel levels for the session
	Levels() LevelsMap
	// ExchangeScratchArea returns the scratch area for exchanges.
	ExchangeScratchArea() *ExchangesScratchArea
}

// Session aborted error.
type ErrAbort struct {
	Reason string
}

func (ea *ErrAbort) Error() string {
	return fmt.Sprintf("session aborted (%s)", ea.Reason)
}

// BrokerConfig gives access to the typical broker configuration.
type BrokerConfig interface {
	// SessionQueueSize gives the session queue size.
	SessionQueueSize() uint
	// BrokerQueueSize gives the internal broker queue size.
	BrokerQueueSize() uint
}
