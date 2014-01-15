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

// representative struct for messages

import (
	"encoding/json"
	// "log"
)

//  System channel id using a shortened hex encoded form for the NIL UUID
const SystemChannelId = "0"

// CONNECT message
type ConnectMsg struct {
	Type      string `json:"T"`
	ClientVer string
	DeviceId  string
	Info      map[string]interface{} `json:",omitempty"` // platform etc...
	// maps channel ids (hex encoded UUIDs) to known client channel levels
	Levels map[string]int64
}

// CONNACK message
type ConnAckMsg struct {
	Type   string `json:"T"`
	Params ConnAckParams
}

// ConnAckParams carries the connection parameters from the server on
// connection acknowledment.
type ConnAckParams struct {
	// ping interval formatted time.Duration
	PingInterval string
}

// PING/PONG messages
type PingPongMsg struct {
	Type string `json:"T"`
}

const maxPayloadSize = 62 * 1024

// SplittableMsg are messages that may require and are capable of splitting.
type SplittableMsg interface {
	Split() (done bool)
}

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

// ACKnowelgement message
type AckMsg struct {
	Type string `json:"T"`
}

// xxx ... query levels messages
