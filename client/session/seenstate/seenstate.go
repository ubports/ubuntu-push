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

// Package seenstate holds implementations of the SeenState that the client
// session uses to keep track of what messages it has seen.
package seenstate

import (
	"github.com/ubports/ubuntu-push/protocol"
)

type SeenState interface {
	// Set() (re)sets the given level to the given value.
	SetLevel(level string, top int64) error
	// GetAll() returns a "simple" map of the current levels.
	GetAllLevels() (map[string]int64, error)
	// FilterBySeen filters notifications already seen, keep track
	// of them as well.
	FilterBySeen([]protocol.Notification) ([]protocol.Notification, error)
	// Close closes state.
	Close()
}

type memSeenState struct {
	levels   map[string]int64
	seenMsgs map[string]bool
}

func (m *memSeenState) SetLevel(level string, top int64) error {
	m.levels[level] = top
	return nil
}
func (m *memSeenState) GetAllLevels() (map[string]int64, error) {
	return m.levels, nil
}

func (m *memSeenState) FilterBySeen(notifs []protocol.Notification) ([]protocol.Notification, error) {
	acc := make([]protocol.Notification, 0, len(notifs))
	for _, notif := range notifs {
		seen := m.seenMsgs[notif.MsgId]
		if seen {
			continue
		}
		m.seenMsgs[notif.MsgId] = true
		acc = append(acc, notif)
	}
	return acc, nil
}

func (m *memSeenState) Close() {
}

var _ SeenState = (*memSeenState)(nil)

// NewSeenState returns an implementation of SeenState that is memory-based and
// does not save state.
func NewSeenState() (SeenState, error) {
	return &memSeenState{
		levels:   make(map[string]int64),
		seenMsgs: make(map[string]bool),
	}, nil
}
