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

package testing

import (
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/testing/condition"
	"testing"
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type NMTSuite struct{}

var _ = Suite(&NMTSuite{})

// Test that Call() with a positive condition returns the first return value
// provided, as advertised.
func (s *NMTSuite) TestCallReturnsFirstRetval(c *C) {
	var m, n uint32 = 42, 17
	tc := New(condition.Work(true), m, n)
	v, e := tc.Call("what")
	c.Check(e, IsNil)
	c.Check(v, Equals, m)
}

// Test that Call() with a negative condition returns an error.
func (s *NMTSuite) TestCallFails(c *C) {
	tc := New(condition.Work(false))
	_, e := tc.Call("what")
	c.Check(e, NotNil)
}

// Test that Call() with a positive condition and no return values panics with
// a helpful message.
func (s *NMTSuite) TestCallPanicsWithNiceMessage(c *C) {
	tc := New(condition.Work(true))
	c.Check(func() { tc.Call("") }, PanicMatches, "No return values provided!")
}

// Test that WatchSignal() with a positive condition sends the provided return
// values over the channel.
func (s *NMTSuite) TestWatch(c *C) {
	var m, n uint32 = 42, 17
	tc := New(condition.Work(true), m, n)
	ch := make(chan uint32)
	e := tc.WatchSignal("what", func(u interface{}) { ch <- u.(uint32) }, func() { close(ch) })
	c.Check(e, IsNil)
	c.Check(<-ch, Equals, m)
	c.Check(<-ch, Equals, n)
}

// Test that WatchSignal() with a negative condition returns an error.
func (s *NMTSuite) TestWatchFails(c *C) {
	tc := New(condition.Work(false))
	e := tc.WatchSignal("what", func(u interface{}) {}, func() {})
	c.Check(e, NotNil)
}
