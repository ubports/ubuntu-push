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

type SeenState interface {
	// Set() (re)sets the given level to the given value.
	SetLevel(level string, top int64) error
	// GetAll() returns a "simple" map of the current levels.
	GetAllLevels() (map[string]int64, error)
}

type mapLevelMap map[string]int64

func (m *mapLevelMap) SetLevel(level string, top int64) error {
	(*m)[level] = top
	return nil
}
func (m *mapLevelMap) GetAllLevels() (map[string]int64, error) {
	return map[string]int64(*m), nil
}

var _ SeenState = (*mapLevelMap)(nil)

// NewSeenState returns an implementation of SeenState that is memory-based and
// does not save state.
func NewSeenState() (SeenState, error) {
	return &mapLevelMap{}, nil
}
