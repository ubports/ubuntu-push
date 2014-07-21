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
	"fmt"
	"strings"

	. "launchpad.net/gocheck"
)

type mBoxSuite struct {
	prevMBoxMaxMessagesSize int
}

var _ = Suite(&mBoxSuite{})

func (s *mBoxSuite) SetUpSuite(c *C) {
	s.prevMBoxMaxMessagesSize = mBoxMaxMessagesSize
	mBoxMaxMessagesSize = 100
}

func (s *mBoxSuite) TearDownSuite(c *C) {
	mBoxMaxMessagesSize = s.prevMBoxMaxMessagesSize
}

func (s *mBoxSuite) TestAppend(c *C) {
	mbox := &mBox{}
	m1 := json.RawMessage(`{"m":1}`)
	m2 := json.RawMessage(`{"m":2}`)
	mbox.Append(m1, "n1")
	mbox.Append(m2, "n2")
	c.Check(mbox.messages, DeepEquals, []string{string(m1), string(m2)})
	c.Check(mbox.nids, DeepEquals, []string{"n1", "n2"})
}

func (s *mBoxSuite) TestAllMessagesEmpty(c *C) {
	mbox := &mBox{}
	c.Check(mbox.AllMessages(), HasLen, 0)
}

func (s *mBoxSuite) TestAllMessages(c *C) {
	mbox := &mBox{}
	m1 := json.RawMessage(`{"m":1}`)
	m2 := json.RawMessage(`{"m":2}`)
	mbox.Append(m1, "n1")
	mbox.Append(m2, "n2")
	c.Check(mbox.AllMessages(), DeepEquals, []string{string(m1), string(m2)})
}

func blobMessage(n int, sz int) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"n":%d,"b":"%s"}`, n, strings.Repeat("x", sz-14)))
}

func (s *mBoxSuite) TestAppendEvictSome(c *C) {
	mbox := &mBox{}
	m1 := blobMessage(1, 25)
	m2 := blobMessage(2, 25)
	m3 := blobMessage(3, 50)
	mbox.Append(m1, "n1")
	mbox.Append(m2, "n2")
	mbox.Append(m3, "n3")
	c.Check(mbox.curSize, Equals, 100)
	c.Check(mbox.evicted, Equals, 0)
	m4 := blobMessage(4, 23)
	mbox.Append(m4, "n4")
	c.Assert(mbox.evicted, Equals, 1)
	c.Check(mbox.curSize, Equals, 25+50+23)
	c.Check(mbox.AllMessages(), DeepEquals, []string{string(m2), string(m3), string(m4)})
	c.Check(mbox.messages[:1], DeepEquals, []string{""})
	c.Check(mbox.nids, DeepEquals, []string{"", "n2", "n3", "n4"})
}

func (s *mBoxSuite) TestAppendEvictSomeCopyOver(c *C) {
	mbox := &mBox{}
	m1 := blobMessage(1, 25)
	m2 := blobMessage(2, 25)
	m3 := blobMessage(3, 25)
	m4 := blobMessage(4, 25)
	mbox.Append(m1, "n1")
	mbox.Append(m2, "n2")
	mbox.Append(m3, "n3")
	mbox.Append(m4, "n4")
	c.Check(mbox.curSize, Equals, 100)
	c.Check(mbox.evicted, Equals, 0)
	m5 := blobMessage(5, 40)
	mbox.Append(m5, "n5")
	c.Assert(mbox.evicted, Equals, 0)
	c.Check(mbox.curSize, Equals, 90)
	c.Check(mbox.AllMessages(), DeepEquals, []string{string(m3), string(m4), string(m5)})
	c.Check(mbox.nids, DeepEquals, []string{"n3", "n4", "n5"})
	// do it again
	m6 := blobMessage(6, 40)
	mbox.Append(m6, "n6")
	c.Assert(mbox.evicted, Equals, 0)
	c.Check(mbox.curSize, Equals, 80)
	c.Check(mbox.AllMessages(), DeepEquals, []string{string(m5), string(m6)})
	c.Check(mbox.nids, DeepEquals, []string{"n5", "n6"})
}

func (s *mBoxSuite) TestAppendEvictEverything(c *C) {
	mbox := &mBox{}
	m1 := blobMessage(1, 25)
	m2 := blobMessage(2, 25)
	m3 := blobMessage(3, 50)
	mbox.Append(m1, "n1")
	mbox.Append(m2, "n2")
	mbox.Append(m3, "n3")
	c.Check(mbox.curSize, Equals, 100)
	c.Check(mbox.evicted, Equals, 0)
	m4 := blobMessage(4, 90)
	mbox.Append(m4, "n4")
	c.Assert(mbox.evicted, Equals, 0)
	c.Check(mbox.curSize, Equals, 90)
	c.Check(mbox.AllMessages(), DeepEquals, []string{string(m4)})
	c.Check(mbox.nids, DeepEquals, []string{"n4"})
}
