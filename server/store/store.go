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

// Package store takes care of storing pending notifications.
package store

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"launchpad.net/ubuntu-push/protocol"
)

type InternalChannelId string

func (icid InternalChannelId) BroadcastChannel() bool {
	marker := icid[0]
	return marker == 'B' || marker == '0'
}

var ErrUnknownChannel = errors.New("unknown channel name")
var ErrFull = errors.New("channel is full")
var ErrExpected128BitsHexRepr = errors.New("expected 128 bits hex repr")

const SystemInternalChannelId = InternalChannelId("0")

func InternalChannelIdToHex(chanId InternalChannelId) string {
	if chanId == SystemInternalChannelId {
		return "0"
	}
	if !chanId.BroadcastChannel() {
		panic("InternalChannelIdToHex is for broadcast channels")
	}
	return hex.EncodeToString([]byte(chanId)[1:])
}

var zero128 [16]byte

const noId = InternalChannelId("")

func HexToInternalChannelId(hexRepr string) (InternalChannelId, error) {
	if hexRepr == "0" {
		return SystemInternalChannelId, nil
	}
	if len(hexRepr) != 32 {
		return noId, ErrExpected128BitsHexRepr
	}
	var idbytes [16]byte
	_, err := hex.Decode(idbytes[:], []byte(hexRepr))
	if err != nil {
		return noId, ErrExpected128BitsHexRepr
	}
	if idbytes == zero128 {
		return SystemInternalChannelId, nil
	}
	// mark with B(roadcast) prefix
	s := "B" + string(idbytes[:])
	return InternalChannelId(s), nil
}

// PendingStore let store notifications into channels.
type PendingStore interface {
	// GetInternalChannelId returns the internal store id for a channel
	// given the name.
	GetInternalChannelId(name string) (InternalChannelId, error)
	// AppendToChannel appends a notification to the channel.
	AppendToChannel(chanId InternalChannelId, notification json.RawMessage, expiration time.Time) error
	// GetChannelSnapshot gets all the current notifications and
	// current top level in the channel.
	GetChannelSnapshot(chanId InternalChannelId) (topLevel int64, notifications []protocol.Notification, err error)
	// Close is to be called when done with the store.
	Close()
}
