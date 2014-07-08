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

package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"launchpad.net/ubuntu-push/protocol"
)

// one stored channel
type channel struct {
	topLevel      int64
	notifications []protocol.Notification
	expirations   []time.Time
}

// InMemoryPendingStore is a basic in-memory pending notification store.
type InMemoryPendingStore struct {
	lock  sync.Mutex
	store map[InternalChannelId]*channel
}

// NewInMemoryPendingStore returns a new InMemoryStore.
func NewInMemoryPendingStore() *InMemoryPendingStore {
	return &InMemoryPendingStore{
		store: make(map[InternalChannelId]*channel),
	}
}

func (sto *InMemoryPendingStore) Register(deviceId, appId string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s::%s", appId, deviceId))), nil
}

func (sto *InMemoryPendingStore) Unregister(deviceId, appId string) error {
	// do nothing, tokens here are computed deterministically and not stored
	return nil
}

func (sto *InMemoryPendingStore) GetInternalChannelIdFromToken(token, appId, userId, deviceId string) (InternalChannelId, error) {
	if token != "" && appId != "" {
		decoded, err := base64.StdEncoding.DecodeString(token)
		if err != nil {
			return "", ErrUnknownToken
		}
		token = string(decoded)
		if !strings.HasPrefix(token, appId+"::") {
			return "", ErrUnauthorized
		}
		deviceId := token[len(appId)+2:]
		return UnicastInternalChannelId(deviceId, deviceId), nil
	}
	if userId != "" && deviceId != "" {
		return UnicastInternalChannelId(userId, deviceId), nil
	}
	return "", ErrUnknownToken
}

func (sto *InMemoryPendingStore) GetInternalChannelId(name string) (InternalChannelId, error) {
	if name == "system" {
		return SystemInternalChannelId, nil
	}
	return InternalChannelId(""), ErrUnknownChannel
}

func (sto *InMemoryPendingStore) appendToChannel(chanId InternalChannelId, newNotification protocol.Notification, inc int64, expiration time.Time) error {
	sto.lock.Lock()
	defer sto.lock.Unlock()
	prev := sto.store[chanId]
	if prev == nil {
		prev = &channel{}
	}
	prev.topLevel += inc
	prev.notifications = append(prev.notifications, newNotification)
	prev.expirations = append(prev.expirations, expiration)
	sto.store[chanId] = prev
	return nil
}

func (sto *InMemoryPendingStore) AppendToChannel(chanId InternalChannelId, notificationPayload json.RawMessage, expiration time.Time) error {
	newNotification := protocol.Notification{Payload: notificationPayload}
	return sto.appendToChannel(chanId, newNotification, 1, expiration)
}

func (sto *InMemoryPendingStore) AppendToUnicastChannel(chanId InternalChannelId, appId string, notificationPayload json.RawMessage, msgId string, expiration time.Time) error {
	newNotification := protocol.Notification{
		Payload: notificationPayload,
		AppId:   appId,
		MsgId:   msgId,
	}
	return sto.appendToChannel(chanId, newNotification, 0, expiration)
}

func (sto *InMemoryPendingStore) getChannelUnfiltered(chanId InternalChannelId) (*channel, []protocol.Notification, []Metadata) {
	channel, ok := sto.store[chanId]
	if !ok {
		return nil, nil, nil
	}
	n := len(channel.notifications)
	res := make([]protocol.Notification, n)
	meta := make([]Metadata, n)
	for i, notif := range channel.notifications {
		res[i] = notif
		meta[i] = Metadata{Expiration: channel.expirations[i]}
	}
	return channel, res, meta
}

func (sto *InMemoryPendingStore) GetChannelUnfiltered(chanId InternalChannelId) (int64, []protocol.Notification, []Metadata, error) {
	sto.lock.Lock()
	defer sto.lock.Unlock()
	channel, res, meta := sto.getChannelUnfiltered(chanId)
	if channel == nil {
		return 0, nil, nil, nil
	}
	return channel.topLevel, res, meta, nil
}

func FilterNotifications(notifications []protocol.Notification, meta []Metadata) []protocol.Notification {
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

func (sto *InMemoryPendingStore) GetChannelSnapshot(chanId InternalChannelId) (int64, []protocol.Notification, error) {
	topLevel, res, meta, _ := sto.GetChannelUnfiltered(chanId)
	if res == nil {
		return 0, nil, nil
	}
	res = FilterNotifications(res, meta)
	return topLevel, res, nil
}

func (sto *InMemoryPendingStore) Scrub(chanId InternalChannelId, appId string) error {
	sto.lock.Lock()
	defer sto.lock.Unlock()
	channel, res, meta := sto.getChannelUnfiltered(chanId)
	if channel == nil {
		return nil
	}
	fresh := FilterNotifications(res, meta)
	res = make([]protocol.Notification, 0, len(fresh))
	exps := make([]time.Time, 0, len(fresh))
	i := 0
	for j := range meta {
		if meta[j].Obsolete {
			continue
		}
		notif := fresh[i]
		i++
		if notif.AppId == appId {
			continue
		}
		res = append(res, notif)
		exps = append(exps, meta[j].Expiration)
	}
	// store as well
	channel.notifications = res
	channel.expirations = exps
	return nil
}

func (sto *InMemoryPendingStore) Close() {
	// ignored
}

func (sto *InMemoryPendingStore) DropByMsgId(chanId InternalChannelId, targets []protocol.Notification) error {
	sto.lock.Lock()
	defer sto.lock.Unlock()
	channel, ok := sto.store[chanId]
	if !ok {
		return nil
	}
	expById := make(map[string]time.Time, len(channel.notifications))
	for i, notif := range channel.notifications {
		expById[notif.MsgId] = channel.expirations[i]
	}
	channel.notifications = FilterOutByMsgId(channel.notifications, targets)
	exps := make([]time.Time, len(channel.notifications))
	for i, notif := range channel.notifications {
		exps[i] = expById[notif.MsgId]
	}
	channel.expirations = exps
	return nil
}

// sanity check we implement the interface
var _ PendingStore = (*InMemoryPendingStore)(nil)
