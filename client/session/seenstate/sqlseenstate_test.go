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

	_ "github.com/mattn/go-sqlite3"
	. "launchpad.net/gocheck"

	"github.com/ubports/ubuntu-push/protocol"
)

type sqlsSuite struct{ ssSuite }

var _ = Suite(&sqlsSuite{})

func (s *sqlsSuite) SetUpSuite(c *C) {
	s.constructor = func() (SeenState, error) { return NewSqliteSeenState(":memory:") }
}

func (s *sqlsSuite) TestNewCanFail(c *C) {
	sqls, err := NewSqliteSeenState("/does/not/exist")
	c.Assert(sqls, IsNil)
	c.Check(err, NotNil)
}

func (s *sqlsSuite) TestSetCanFail(c *C) {
	dir := c.MkDir()
	filename := dir + "test.db"
	db, err := sql.Open("sqlite3", filename)
	c.Assert(err, IsNil)
	// create the wrong kind of table
	_, err = db.Exec("CREATE TABLE level_map (foo)")
	c.Assert(err, IsNil)
	// <evil laughter>
	sqls, err := NewSqliteSeenState(filename)
	c.Check(err, IsNil)
	c.Assert(sqls, NotNil)
	err = sqls.SetLevel("foo", 42)
	c.Check(err, ErrorMatches, "cannot set .*")
}

func (s *sqlsSuite) TestGetAllCanFail(c *C) {
	dir := c.MkDir()
	filename := dir + "test.db"
	db, err := sql.Open("sqlite3", filename)
	c.Assert(err, IsNil)
	// create the wrong kind of table
	_, err = db.Exec("CREATE TABLE level_map AS SELECT 'what'")
	c.Assert(err, IsNil)
	// <evil laughter>
	sqls, err := NewSqliteSeenState(filename)
	c.Check(err, IsNil)
	c.Assert(sqls, NotNil)
	all, err := sqls.GetAllLevels()
	c.Check(all, IsNil)
	c.Check(err, ErrorMatches, "cannot read level .*")
}

func (s *sqlsSuite) TestGetAllCanFailDifferently(c *C) {
	dir := c.MkDir()
	filename := dir + "test.db"
	db, err := sql.Open("sqlite3", filename)
	c.Assert(err, IsNil)
	// create a view with the name the table will have
	_, err = db.Exec("CREATE TABLE foo (foo)")
	c.Assert(err, IsNil)
	_, err = db.Exec("CREATE VIEW level_map AS SELECT * FROM foo")
	c.Assert(err, IsNil)
	// break the view
	_, err = db.Exec("DROP TABLE foo")
	c.Assert(err, IsNil)
	// <evil laughter>
	sqls, err := NewSqliteSeenState(filename)
	c.Check(err, IsNil)
	c.Assert(sqls, NotNil)
	all, err := sqls.GetAllLevels()
	c.Check(all, IsNil)
	c.Check(err, ErrorMatches, "cannot retrieve levels .*")
}

func (s *sqlsSuite) TestFilterBySeenCanFail(c *C) {
	dir := c.MkDir()
	filename := dir + "test.db"
	db, err := sql.Open("sqlite3", filename)
	c.Assert(err, IsNil)
	// create the wrong kind of table
	_, err = db.Exec("CREATE TABLE seen_msgs AS SELECT 'what'")
	c.Assert(err, IsNil)
	// <evil laughter>
	sqls, err := NewSqliteSeenState(filename)
	c.Check(err, IsNil)
	c.Assert(sqls, NotNil)
	n1 := protocol.Notification{MsgId: "m1"}
	res, err := sqls.FilterBySeen([]protocol.Notification{n1})
	c.Check(res, IsNil)
	c.Check(err, ErrorMatches, "cannot insert .*")
}

func (s *sqlsSuite) TestClose(c *C) {
	dir := c.MkDir()
	filename := dir + "test.db"
	sqls, err := NewSqliteSeenState(filename)
	c.Check(err, IsNil)
	c.Assert(sqls, NotNil)
	sqls.Close()
}

func (s *sqlsSuite) TestDropPrevThan(c *C) {
	dir := c.MkDir()
	filename := dir + "test.db"
	db, err := sql.Open("sqlite3", filename)
	c.Assert(err, IsNil)
	sqls, err := NewSqliteSeenState(filename)
	c.Check(err, IsNil)
	c.Assert(sqls, NotNil)

	_, err = db.Exec("INSERT INTO seen_msgs (id) VALUES (?)", "m1")
	c.Assert(err, IsNil)
	_, err = db.Exec("INSERT INTO seen_msgs (id) VALUES (?)", "m2")
	c.Assert(err, IsNil)
	_, err = db.Exec("INSERT INTO seen_msgs (id) VALUES (?)", "m3")
	c.Assert(err, IsNil)
	_, err = db.Exec("INSERT INTO seen_msgs (id) VALUES (?)", "m4")
	c.Assert(err, IsNil)
	_, err = db.Exec("INSERT INTO seen_msgs (id) VALUES (?)", "m5")
	c.Assert(err, IsNil)

	rows, err := db.Query("SELECT COUNT(*) FROM seen_msgs")
	c.Assert(err, IsNil)
	rows.Next()
	var i int
	err = rows.Scan(&i)
	c.Assert(err, IsNil)
	c.Check(i, Equals, 5)
	rows.Close()

	err = sqls.(*sqliteSeenState).dropPrevThan("m3")
	c.Assert(err, IsNil)

	rows, err = db.Query("SELECT COUNT(*) FROM seen_msgs")
	c.Assert(err, IsNil)
	rows.Next()
	err = rows.Scan(&i)
	c.Assert(err, IsNil)
	c.Check(i, Equals, 3)
	rows.Close()

	var msgId string
	rows, err = db.Query("SELECT * FROM seen_msgs")
	rows.Next()
	err = rows.Scan(&msgId)
	c.Assert(err, IsNil)
	c.Check(msgId, Equals, "m3")
	rows.Next()
	err = rows.Scan(&msgId)
	c.Assert(err, IsNil)
	c.Check(msgId, Equals, "m4")
	rows.Next()
	err = rows.Scan(&msgId)
	c.Assert(err, IsNil)
	c.Check(msgId, Equals, "m5")
	rows.Close()
}
