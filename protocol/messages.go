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

// Structs representing messages.

import (
	"encoding/json"
)

// System channel id using a shortened hex-encoded form for the NIL UUID.
const SystemChannelId = "0"

// CONNECT message
type ConnectMsg struct {
	Type          string `json:"T"`
	ClientVer     string
	DeviceId      string
	Authorization string
	Info          map[string]interface{} `json:",omitempty"` // platform etc...
	// maps channel ids (hex encoded UUIDs) to known client channel levels
	Levels map[string]int64
}

// CONNACK message
type ConnAckMsg struct {
	Type   string `json:"T"`
	Params ConnAckParams
}

// ConnAckParams carries the connection parameters from the server on
// connection acknowledgement.
type ConnAckParams struct {
	// ping interval formatted time.Duration
	PingInterval string
}

// SplittableMsg are messages that may require and are capable of splitting.
type SplittableMsg interface {
	Split() (done bool)
}

// OnewayMsg are messages that are not to be followed by a response,
// after sending them the session either abort or continue.
type OnewayMsg interface {
	SplittableMsg
	// continue session after the message?
	OnewayContinue() bool
}

// CONNBROKEN message, server side is breaking the connection for reason.
type ConnBrokenMsg struct {
	Type string `json:"T"`
	// reason
	Reason string
}

func (m *ConnBrokenMsg) Split() bool {
	return true
}

func (m *ConnBrokenMsg) OnewayContinue() bool {
	return false
}

// CONNBROKEN reasons
const (
	BrokenHostMismatch = "host-mismatch"
)

// CONNWARN message, server side is warning about partial functionality
// because reason.
type ConnWarnMsg struct {
	Type string `json:"T"`
	// reason
	Reason string
}

func (m *ConnWarnMsg) Split() bool {
	return true
}
func (m *ConnWarnMsg) OnewayContinue() bool {
	return true
}

// CONNWARN reasons
const (
	WarnUnauthorized = "unauthorized"
)

// PING/PONG messages
type PingPongMsg struct {
	Type string `json:"T"`
}

const maxPayloadSize = 62 * 1024

// BROADCAST messages
type BroadcastMsg struct {
	Type      string `json:"T"`
	AppId     string `json:",omitempty"`
	ChanId    string
	TopLevel  int64
	Payloads  []json.RawMessage
	splitting int
}

func (m *BroadcastMsg) Split() bool {
	var prevTop int64
	if m.splitting == 0 {
		prevTop = m.TopLevel - int64(len(m.Payloads))
	} else {
		prevTop = m.TopLevel
		m.Payloads = m.Payloads[len(m.Payloads):m.splitting]
		m.TopLevel = prevTop + int64(len(m.Payloads))
	}
	payloads := m.Payloads
	var size int
	for i := range payloads {
		size += len(payloads[i])
		if size > maxPayloadSize {
			m.TopLevel = prevTop + int64(i)
			m.splitting = len(payloads)
			m.Payloads = payloads[:i]
			return false
		}
	}
	return true
}

// Reset resets the splitting state if the message storage is to be
// reused.
func (b *BroadcastMsg) Reset() {
	b.splitting = 0
}

// NOTIFICATIONS message
type NotificationsMsg struct {
	Type          string `json:"T"`
	Notifications []Notification
}

// A single unicast notification
type Notification struct {
	AppId string `json:"A"`
	MsgId string `json:"M"`
	// payload
	Payload json.RawMessage `json:"P"`
}

// ACKnowledgement message
type AckMsg struct {
	Type string `json:"T"`
}

// xxx ... query levels messages
