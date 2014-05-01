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
	"encoding/json"
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

func (sto *InMemoryPendingStore) GetChannelSnapshot(chanId InternalChannelId) (int64, []protocol.Notification, error) {
	sto.lock.Lock()
	defer sto.lock.Unlock()
	channel, ok := sto.store[chanId]
	if !ok {
		return 0, nil, nil
	}
	topLevel := channel.topLevel
	n := len(channel.notifications)
	res := make([]protocol.Notification, 0, n)
	exps := make([]time.Time, 0, n)
	now := time.Now()
	for i, expiration := range channel.expirations {
		if expiration.Before(now) {
			continue
		}
		res = append(res, channel.notifications[i])
		exps = append(exps, expiration)
	}
	// store as well
	channel.notifications = res
	channel.expirations = exps
	return topLevel, res, nil
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
	channel.notifications = DropByMsgId(channel.notifications, targets)
	exps := make([]time.Time, len(channel.notifications))
	for i, notif := range channel.notifications {
		exps[i] = expById[notif.MsgId]
	}
	channel.expirations = exps
	return nil
}

// sanity check we implement the interface
var _ PendingStore = (*InMemoryPendingStore)(nil)
