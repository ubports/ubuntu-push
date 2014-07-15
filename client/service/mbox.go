/*
 Copyright 2014 Canonical Ltd.

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

package service

import (
	"encoding/json"
)

var mBoxMaxMessagesSize = 128 * 1024

// mBox can hold a size-limited amount of notification messages for one application.
type mBox struct {
	evicted  int
	curSize  int
	messages []string
	nids     []string
}

func (box *mBox) evictFor(sz int) {
	evictedSize := 0
	i := box.evicted
	n := len(box.messages)
	for evictedSize < sz && i < n {
		evictedSize += len(box.messages[i])
		box.evicted++
		i++
	}
	box.curSize -= evictedSize
}

// Append appends a message with notification id to the mbox.
func (box *mBox) Append(message json.RawMessage, nid string) {
	sz := len(message)
	if box.curSize+sz > mBoxMaxMessagesSize {
		// make space
		box.evictFor(sz)
	}
	n := len(box.messages)
	evicted := box.evicted
	if evicted > 0 {
		if evicted == n {
			box.messages = box.messages[0:0]
			box.nids = box.nids[0:0]
			box.evicted = 0
		} else if n == cap(box.messages)-1 {
			// here we would get a resize and copy anyway
			copy(box.messages, box.messages[box.evicted:])
			kept := n - box.evicted
			box.messages = box.messages[0:kept]
			copy(box.nids, box.nids[box.evicted:])
			box.nids = box.nids[0:kept]
			box.evicted = 0
		}
	}
	box.messages = append(box.messages, string(message))
	box.nids = append(box.nids, nid)
	box.curSize += sz
}

// AllMessages gets all messages from the mbox.
func (box *mBox) AllMessages() []string {
	return box.messages[box.evicted:]
}
