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
	_ "code.google.com/p/gosqlite/sqlite3"
	"database/sql"
	. "launchpad.net/gocheck"
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
