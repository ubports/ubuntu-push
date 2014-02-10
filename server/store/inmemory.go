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
)

// one stored notification
type notification struct {
	payload json.RawMessage
	expiration time.Time
}

// one stored channel
type channel struct {
	topLevel int64
	notifications []notification
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

func (sto *InMemoryPendingStore) AppendToChannel(chanId InternalChannelId, notificationPayload json.RawMessage, expiration time.Time) error {
	sto.lock.Lock()
	defer sto.lock.Unlock()
	prev := sto.store[chanId]
	if prev == nil {
		prev = &channel{}
	}
	prev.topLevel++
	prev.notifications = append(prev.notifications, notification{
		payload: notificationPayload,
		expiration: expiration,
	})
	sto.store[chanId] = prev
	return nil
}

func (sto *InMemoryPendingStore) GetChannelSnapshot(chanId InternalChannelId) (int64, []json.RawMessage, error) {
	sto.lock.Lock()
	defer sto.lock.Unlock()
	channel, ok := sto.store[chanId]
	if !ok {
		return 0, nil, nil
	}
	topLevel := channel.topLevel
	n := len(channel.notifications)
	res := make([]json.RawMessage, 0, n)
	now := time.Now()
	for _, notification := range channel.notifications {
		if notification.expiration.Before(now) {
			continue
		}
		res = append(res, notification.payload)
	}
	return topLevel, res, nil
}

// sanity check we implement the interface
var _ PendingStore = &InMemoryPendingStore{}
