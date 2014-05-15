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

package seenstate

import (
	"database/sql"
	"fmt"
	"strings"

	_ "code.google.com/p/gosqlite/sqlite3"

	"launchpad.net/ubuntu-push/protocol"
)

type sqliteSeenState struct {
	db *sql.DB
}

// NewSqliteSeenState returns an implementation of SeenState that
// keeps and persists the state in an sqlite database.
func NewSqliteSeenState(filename string) (SeenState, error) {
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open sqlite level map %#v: %v", filename, err)
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS level_map (level text primary key, top integer)")
	if err != nil {
		return nil, fmt.Errorf("cannot (re)create sqlite level map table: %v", err)
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS seen_msgs (id text primary key)")
	if err != nil {
		return nil, fmt.Errorf("cannot (re)create sqlite seen msgs table: %v", err)
	}
	return &sqliteSeenState{db}, nil
}

func (ps *sqliteSeenState) SetLevel(level string, top int64) error {
	_, err := ps.db.Exec("REPLACE INTO level_map (level, top) VALUES (?, ?)", level, top)
	if err != nil {
		return fmt.Errorf("cannot set %#v to %#v in level map: %v", level, top, err)
	}
	return nil
}
func (ps *sqliteSeenState) GetAllLevels() (map[string]int64, error) {
	rows, err := ps.db.Query("SELECT * FROM level_map")
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve levels from sqlite level map: %v", err)
	}
	m := map[string]int64{}
	for rows.Next() {
		var level string
		var top int64
		err = rows.Scan(&level, &top)
		if err != nil {
			return nil, fmt.Errorf("cannot read level from sqlite level map: %v", err)
		}
		m[level] = top
	}
	return m, nil
}

func (ps *sqliteSeenState) dropPrevThan(msgId string) error {
	_, err := ps.db.Exec("DELETE FROM seen_msgs WHERE rowid < (SELECT rowid FROM seen_msgs WHERE id = ?)", msgId)
	return err
}

func (ps *sqliteSeenState) FilterBySeen(notifs []protocol.Notification) ([]protocol.Notification, error) {
	if len(notifs) == 0 {
		return nil, nil
	}
	acc := make([]protocol.Notification, 0, len(notifs))
	for _, notif := range notifs {
		_, err := ps.db.Exec("INSERT INTO seen_msgs (id) VALUES (?)", notif.MsgId)
		if err != nil {
			if strings.HasSuffix(err.Error(), "UNIQUE constraint failed: seen_msgs.id") {
				continue
			}
			return nil, fmt.Errorf("cannot insert %#v in seen msgs: %v", notif.MsgId, err)
		}
		acc = append(acc, notif)
	}
	err := ps.dropPrevThan(notifs[0].MsgId)
	if err != nil {
		return nil, fmt.Errorf("cannot delete obsolete seen msgs: %v", err)
	}
	return acc, nil
}
