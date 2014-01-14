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

// Package testing contains helpers for testing.
package testing

import (
	"bytes"
	"sync"
)

// SyncedLogBuffer can be used with NewSimpleLogger avoiding races
// when checking the logging done from different goroutines.
type SyncedLogBuffer struct {
	bytes.Buffer
	lock    sync.Mutex
	Written chan bool
}

func (buf *SyncedLogBuffer) Write(b []byte) (int, error) {
	buf.lock.Lock()
	defer buf.lock.Unlock()
	n, err := buf.Buffer.Write(b)
	if buf.Written != nil {
		buf.Written <- true
	}
	return n, err
}

func (buf *SyncedLogBuffer) String() string {
	buf.lock.Lock()
	defer buf.lock.Unlock()
	return buf.Buffer.String()
}
