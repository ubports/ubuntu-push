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
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/testing/condition"
	"testing"
	"time"
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type TestingEndpointSuite struct{}

var _ = Suite(&TestingEndpointSuite{})

// Test that Call() with a positive condition returns the first return value
// provided, as advertised.
func (s *TestingEndpointSuite) TestCallReturnsFirstRetval(c *C) {
	var m, n uint32 = 42, 17
	endp := NewTestingEndpoint(nil, condition.Work(true), m, n)
	var r uint32
	e := endp.Call("what", bus.Args(), &r)
	c.Check(e, IsNil)
	c.Check(r, Equals, m)
}

// Test the same Call() but with multi-valued endpoint
func (s *TestingEndpointSuite) TestMultiValuedCall(c *C) {
	var m, n uint32 = 42, 17
	endp := NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{m}, []interface{}{n})
	var r uint32
	e := endp.Call("what", bus.Args(), &r)
	c.Check(e, IsNil)
	c.Check(r, Equals, m)
}

// Test that Call() with a negative condition returns an error.
func (s *TestingEndpointSuite) TestCallFails(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(false))
	e := endp.Call("what", bus.Args())
	c.Check(e, NotNil)
}

// Test that Call() with a positive condition and no return values panics with
// a helpful message.
func (s *TestingEndpointSuite) TestCallPanicsWithNiceMessage(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(true))
	var x int32
	c.Check(func() { endp.Call("", bus.Args(), &x) }, PanicMatches, "No return values provided.*")
}

// Test that Call() updates callArgs
func (s *TestingEndpointSuite) TestCallArgs(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(true))
	err := endp.Call("what", bus.Args("is", "this", "thing"))
	c.Assert(err, IsNil)
	c.Check(GetCallArgs(endp), DeepEquals,
		[]callArgs{{"what", []interface{}{"is", "this", "thing"}}})
}

// Test that Call() fails but does not explode when asked to return values
// that can't be packed into a dbus message.
func (s *TestingEndpointSuite) TestCallFailsOnBadRetval(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(true), Equals)
	var r uint32
	e := endp.Call("what", bus.Args(), &r)
	c.Check(e, NotNil)
}

// Test that Call() fails but does not explode when given an improper result
// destination (one into which the dbus response can't be stuffed).
func (s *TestingEndpointSuite) TestCallFailsOnBadArg(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(true), 1)
	r := func() {}
	e := endp.Call("what", bus.Args(), &r)
	c.Check(e, NotNil)
}

// Test that WatchSignal() with a positive condition sends the provided return
// values over the channel.
func (s *TestingEndpointSuite) TestWatch(c *C) {
	var m, n uint32 = 42, 17
	endp := NewTestingEndpoint(nil, condition.Work(true), m, n)
	ch := make(chan uint32)
	e := endp.WatchSignal("what", func(us ...interface{}) { ch <- us[0].(uint32) }, func() { close(ch) })
	c.Check(e, IsNil)
	c.Check(<-ch, Equals, m)
	c.Check(<-ch, Equals, n)
}

// Test that WatchSignal() calls the destructor callback when it runs out values
func (s *TestingEndpointSuite) TestWatchDestructor(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(true))
	ch := make(chan uint32)
	e := endp.WatchSignal("what", func(us ...interface{}) {}, func() { close(ch) })
	c.Check(e, IsNil)
	_, ok := <-ch
	c.Check(ok, Equals, false)
}

// Test the endpoint can be closed
func (s *TestingEndpointSuite) TestCloser(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(true))
	endp.Close()
	c.Check(GetCallArgs(endp), DeepEquals, []callArgs{
		{
			Member: "::Close",
			Args:   nil,
		}})
}

// Test that WatchSignal() with a negative condition returns an error.
func (s *TestingEndpointSuite) TestWatchFails(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(false))
	e := endp.WatchSignal("what", func(us ...interface{}) {}, func() {})
	c.Check(e, NotNil)
}

// Test WatchSignal can use the WatchTicker instead of a timeout (if
// the former is not nil)
func (s *TestingEndpointSuite) TestWatchTicker(c *C) {
	watchTicker := make(chan bool, 3)
	watchTicker <- true
	watchTicker <- true
	watchTicker <- true
	c.Assert(len(watchTicker), Equals, 3)

	endp := NewTestingEndpoint(nil, condition.Work(true), 0, 0)
	SetWatchTicker(endp, watchTicker)
	ch := make(chan int)
	e := endp.WatchSignal("what", func(us ...interface{}) {}, func() { close(ch) })
	c.Check(e, IsNil)

	// wait for the destructor to be called
	select {
	case <-time.Tick(10 * time.Millisecond):
		c.Fatal("timed out waiting for close on channel")
	case <-ch:
	}

	// now if all went well, the ticker will have been tuck twice.
	c.Assert(len(watchTicker), Equals, 1)
}

// Tests that GetProperty() works
func (s *TestingEndpointSuite) TestGetProperty(c *C) {
	var m uint32 = 42
	endp := NewTestingEndpoint(nil, condition.Work(true), m)
	v, e := endp.GetProperty("what")
	c.Check(e, IsNil)
	c.Check(v, Equals, m)
}

// Tests that GetProperty() fails, too
func (s *TestingEndpointSuite) TestGetPropertyFails(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(false))
	_, e := endp.GetProperty("what")
	c.Check(e, NotNil)
}

// Tests that GetProperty() also fails if it's fed garbage
func (s *TestingEndpointSuite) TestGetPropertyFailsGargling(c *C) {
	endp := NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{})
	_, e := endp.GetProperty("what")
	c.Check(e, NotNil)
}

// Test Dial() with a non-working bus fails
func (s *TestingBusSuite) TestDialNoWork(c *C) {
	endp := NewTestingEndpoint(condition.Work(false), nil)
	err := endp.Dial()
	c.Check(err, NotNil)
}

// Test testingEndpoints serialize, more or less
func (s *TestingBusSuite) TestEndpointString(c *C) {
	endp := NewTestingEndpoint(condition.Fail2Work(2), nil, "hello there")
	c.Check(endp.String(), Matches, ".*Still Broken.*hello there.*")
}

// Test that GrabName updates callArgs
func (s *TestingEndpointSuite) TestGrabNameUpdatesCallArgs(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(true))
	endp.GrabName(false)
	endp.GrabName(true)
	c.Check(GetCallArgs(endp), DeepEquals, []callArgs{
		{
			Member: "::GrabName",
			Args:   []interface{}{false},
		}, {
			Member: "::GrabName",
			Args:   []interface{}{true},
		}})
}

// Test that Signal updates callArgs
func (s *TestingEndpointSuite) TestSignalUpdatesCallArgs(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(true))
	endp.Signal("hello", "", []interface{}{"world"})
	endp.Signal("hello", "/potato", []interface{}{"there"})
	c.Check(GetCallArgs(endp), DeepEquals, []callArgs{
		{
			Member: "::Signal",
			Args:   []interface{}{"hello", "", []interface{}{"world"}},
		}, {
			Member: "::Signal",
			Args:   []interface{}{"hello", "/potato", []interface{}{"there"}},
		}})
}

// Test that WatchMethod updates callArgs
func (s *TestingEndpointSuite) TestWatchMethodUpdatesCallArgs(c *C) {
	endp := NewTestingEndpoint(nil, condition.Work(true))
	foo := func(string, []interface{}, []interface{}) ([]interface{}, error) { return nil, nil }
	foomp := bus.DispatchMap{"foo": foo}
	endp.WatchMethod(foomp, "/*")
	c.Check(GetCallArgs(endp), DeepEquals, []callArgs{
		{
			Member: "::WatchMethod",
			Args:   []interface{}{foomp, []interface{}(nil)},
		}})
}
