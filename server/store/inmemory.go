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
)

// InMemoryPendingStore is a basic in-memory pending notification store.
type InMemoryPendingStore struct {
	lock  sync.Mutex
	store map[InternalChannelId][]json.RawMessage
}

// NewInMemoryPendingStore returns a new InMemoryStore.
func NewInMemoryPendingStore() *InMemoryPendingStore {
	return &InMemoryPendingStore{
		store: make(map[InternalChannelId][]json.RawMessage),
	}
}

func (sto *InMemoryPendingStore) GetInternalChannelId(name string) (InternalChannelId, error) {
	if name == "system" {
		return SystemInternalChannelId, nil
	}
	return InternalChannelId(""), ErrUnknownChannel
}

func (sto *InMemoryPendingStore) AppendToChannel(chanId InternalChannelId, notification json.RawMessage) error {
	sto.lock.Lock()
	defer sto.lock.Unlock()
	prev := sto.store[chanId]
	sto.store[chanId] = append(prev, notification)
	return nil
}

func (sto *InMemoryPendingStore) GetChannelSnapshot(chanId InternalChannelId) (int64, []json.RawMessage, error) {
	sto.lock.Lock()
	defer sto.lock.Unlock()
	notifications, ok := sto.store[chanId]
	if !ok {
		return 0, nil, nil
	}
	n := len(notifications)
	res := make([]json.RawMessage, n)
	copy(res, notifications)
	return int64(n), res, nil
}

// sanity check we implement the interface
var _ PendingStore = &InMemoryPendingStore{}
