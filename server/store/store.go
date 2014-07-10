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
	"fmt"
	"strings"
	"time"

	"launchpad.net/ubuntu-push/protocol"
)

type InternalChannelId string

// BroadcastChannel returns whether the id represents a broadcast channel.
func (icid InternalChannelId) BroadcastChannel() bool {
	marker := icid[0]
	return marker == 'B' || marker == '0'
}

// UnicastChannel returns whether the id represents a unicast channel.
func (icid InternalChannelId) UnicastChannel() bool {
	marker := icid[0]
	return marker == 'U'
}

// UnicastUserAndDevice returns the user and device ids of a unicast channel.
func (icid InternalChannelId) UnicastUserAndDevice() (userId, deviceId string) {
	if !icid.UnicastChannel() {
		panic("UnicastUserAndDevice is for unicast channels")
	}
	parts := strings.SplitN(string(icid)[1:], ":", 2)
	return parts[0], parts[1]
}

var ErrUnknownChannel = errors.New("unknown channel name")
var ErrUnknownToken = errors.New("unknown token")
var ErrUnauthorized = errors.New("unauthorized")
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
	return string(chanId)[1:]
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
	s := "B" + hexRepr
	return InternalChannelId(s), nil
}

// UnicastInternalChannelId builds a channel id for the userId, deviceId pair.
func UnicastInternalChannelId(userId, deviceId string) InternalChannelId {
	return InternalChannelId(fmt.Sprintf("U%s:%s", userId, deviceId))
}

// Metadata holds the metadata stored for a notification.
type Metadata struct {
	Expiration time.Time
	Obsolete   bool
}

// Before checks whether the expiration date in the metadata is before ref.
func (m *Metadata) Before(ref time.Time) bool {
	return m.Expiration.Before(ref)
}

// PendingStore let store notifications into channels.
type PendingStore interface {
	// Register returns a token for a device id, application id pair.
	Register(deviceId, appId string) (token string, err error)
	// Unregister forgets the token for a device id, application id pair.
	Unregister(deviceId, appId string) error
	// GetInternalChannelId returns the internal store id for a channel
	// given the name.
	GetInternalChannelId(name string) (InternalChannelId, error)
	// AppendToChannel appends a notification to the channel.
	AppendToChannel(chanId InternalChannelId, notification json.RawMessage, expiration time.Time) error
	// GetInternalChannelIdFromToken returns the matching internal store
	// id for a channel given a registered token and application id or
	// directly a device id, user id pair.
	GetInternalChannelIdFromToken(token, appId, userId, deviceId string) (InternalChannelId, error)
	// AppendToUnicastChannel appends a notification to the unicast channel.
	AppendToUnicastChannel(chanId InternalChannelId, appId string, notification json.RawMessage, msgId string, meta Metadata) error
	// GetChannelSnapshot gets all the current notifications and
	// current top level in the channel.
	GetChannelSnapshot(chanId InternalChannelId) (topLevel int64, notifications []protocol.Notification, err error)
	// GetChannelUnfiltered gets all the stored notifications with
	// metadata and current top level in the channel.
	GetChannelUnfiltered(chanId InternalChannelId) (topLevel int64, notifications []protocol.Notification, metadata []Metadata, err error)
	// Scrub removes expired notifications and notifications with
	// application id appId (if != "").
	Scrub(chanId InternalChannelId, appId string) error
	// DropByMsgId drops notifications from a unicast channel
	// based on message ids.
	DropByMsgId(chanId InternalChannelId, targets []protocol.Notification) error
	// Close is to be called when done with the store.
	Close()
}

// FilterOutByMsgId returns the notifications from orig whose msg id is not
// mentioned in targets.
func FilterOutByMsgId(orig, targets []protocol.Notification) []protocol.Notification {
	n := len(orig)
	t := len(targets)
	// common case, removing the continuous head
	if t > 0 && n >= t {
		if targets[0].MsgId == orig[0].MsgId {
			for i := t - 1; i >= 0; i-- {
				if i == 0 {
					return orig[t:]
				}
				if targets[i].MsgId != orig[i].MsgId {
					break
				}
			}
		}
	}
	// slow way
	ids := make(map[string]bool, t)
	for _, target := range targets {
		ids[target.MsgId] = true
	}
	acc := make([]protocol.Notification, 0, n)
	for _, notif := range orig {
		if !ids[notif.MsgId] {
			acc = append(acc, notif)
		}
	}
	return acc
}

// FilterOutObsolete filters out expired notifications based on
// paired meta information.
func FilterOutObsolete(notifications []protocol.Notification, meta []Metadata) []protocol.Notification {
	res := make([]protocol.Notification, 0, len(notifications))
	now := time.Now()
	for i := range meta {
		if meta[i].Before(now) {
			meta[i].Obsolete = true
			continue
		}
		res = append(res, notifications[i])
	}
	return res
}
