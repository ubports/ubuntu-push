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

package levelmap

import (
	_ "code.google.com/p/gosqlite/sqlite3"
	"database/sql"
	. "launchpad.net/gocheck"
)

type sqlmSuite struct{ lmSuite }

var _ = Suite(&sqlmSuite{})

func (s *sqlmSuite) SetUpSuite(c *C) {
	s.constructor = func() (LevelMap, error) { return NewSqliteLevelMap(":memory:") }
}

func (s *sqlmSuite) TestNewCanFail(c *C) {
	m, err := NewSqliteLevelMap("/does/not/exist")
	c.Assert(m, IsNil)
	c.Check(err, NotNil)
}

func (s *sqlmSuite) TestSetCanFail(c *C) {
	dir := c.MkDir()
	filename := dir + "test.db"
	db, err := sql.Open("sqlite3", filename)
	c.Assert(err, IsNil)
	// create the wrong kind of table
	_, err = db.Exec("CREATE TABLE level_map (foo)")
	c.Assert(err, IsNil)
	// <evil laughter>
	m, err := NewSqliteLevelMap(filename)
	c.Check(err, IsNil)
	c.Assert(m, NotNil)
	err = m.Set("foo", 42)
	c.Check(err, ErrorMatches, "cannot set .*")
}

func (s *sqlmSuite) TestGetAllCanFail(c *C) {
	dir := c.MkDir()
	filename := dir + "test.db"
	db, err := sql.Open("sqlite3", filename)
	c.Assert(err, IsNil)
	// create the wrong kind of table
	_, err = db.Exec("CREATE TABLE level_map AS SELECT 'what'")
	c.Assert(err, IsNil)
	// <evil laughter>
	m, err := NewSqliteLevelMap(filename)
	c.Check(err, IsNil)
	c.Assert(m, NotNil)
	all, err := m.GetAll()
	c.Check(all, IsNil)
	c.Check(err, ErrorMatches, "cannot read level .*")
}

func (s *sqlmSuite) TestGetAllCanFailDifferently(c *C) {
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
	m, err := NewSqliteLevelMap(filename)
	c.Check(err, IsNil)
	c.Assert(m, NotNil)
	all, err := m.GetAll()
	c.Check(all, IsNil)
	c.Check(err, ErrorMatches, "cannot retrieve levels .*")
}
