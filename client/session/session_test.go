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

package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
	"net"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestSession(t *testing.T) { TestingT(t) }

type clientSessionSuite struct{}

var nullog = logger.NewSimpleLogger(ioutil.Discard, "error")
var debuglog = logger.NewSimpleLogger(os.Stderr, "debug")
var _ = Suite(&clientSessionSuite{})

//
// helpers! candidates to live in their own ../testing/ package.
//

type xAddr string

func (x xAddr) Network() string { return "<:>" }
func (x xAddr) String() string  { return string(x) }

// testConn (roughly based on the one in protocol_test)

type testConn struct {
	Name              string
	Deadlines         []time.Duration
	Writes            [][]byte
	WriteCondition    condition.Interface
	DeadlineCondition condition.Interface
	CloseCondition    condition.Interface
}

func (tc *testConn) LocalAddr() net.Addr { return xAddr(tc.Name) }

func (tc *testConn) RemoteAddr() net.Addr { return xAddr(tc.Name) }

func (tc *testConn) Close() error {
	if tc.CloseCondition == nil || tc.CloseCondition.OK() {
		return nil
	} else {
		return errors.New("closer on fire")
	}
}

func (tc *testConn) SetDeadline(t time.Time) error      { panic("SetDeadline not implemented.") }
func (tc *testConn) SetReadDeadline(t time.Time) error  { panic("SetReadDeadline not implemented.") }
func (tc *testConn) SetWriteDeadline(t time.Time) error { panic("SetWriteDeadline not implemented.") }
func (tc *testConn) Read(buf []byte) (n int, err error) { panic("Read not implemented.") }
func (tc *testConn) Write(buf []byte) (int, error)      { panic("Write not implemented.") }

// test protocol (from session_test)

type testProtocol struct {
	up   chan interface{}
	down chan interface{}
}

// takeNext takes a value from given channel with a 5s timeout
func takeNext(ch <-chan interface{}) interface{} {
	select {
	case <-time.After(5 * time.Second):
		panic("test protocol exchange stuck: too long waiting")
	case v := <-ch:
		return v
	}
	return nil
}

func (c *testProtocol) SetDeadline(t time.Time) {
	deadAfter := t.Sub(time.Now())
	deadAfter = (deadAfter + time.Millisecond/2) / time.Millisecond * time.Millisecond
	c.down <- fmt.Sprintf("deadline %v", deadAfter)
}

func (c *testProtocol) ReadMessage(dest interface{}) error {
	panic("ReadMessage not implemented.")
}

func (c *testProtocol) WriteMessage(src interface{}) error {
	// make sure JSON.Marshal works with src
	_, err := json.Marshal(src)
	if err != nil {
		return err
	}
	val := reflect.ValueOf(src)
	if val.Kind() == reflect.Ptr {
		src = val.Elem().Interface()
	}
	c.down <- src
	switch v := takeNext(c.up).(type) {
	case error:
		return v
	}
	return nil
}

/****************************************************************
  NewSession() tests
****************************************************************/

func (cs *clientSessionSuite) TestNewSessionPlainWorks(c *C) {
	sess, err := NewSession("", nil, 0, "", nullog)
	c.Check(sess, NotNil)
	c.Check(err, IsNil)
	// but no root CAs set
	c.Check(sess.TLS.RootCAs, IsNil)
}

var certfile string = helpers.SourceRelative("../../server/acceptance/config/testing.cert")
var pem, _ = ioutil.ReadFile(certfile)

func (cs *clientSessionSuite) TestNewSessionPEMWorks(c *C) {
	sess, err := NewSession("", pem, 0, "wah", nullog)
	c.Check(sess, NotNil)
	c.Assert(err, IsNil)
	c.Check(sess.TLS.RootCAs, NotNil)
}

func (cs *clientSessionSuite) TestNewSessionBadPEMFileContentFails(c *C) {
	badpem := []byte("This is not the PEM you're looking for.")
	sess, err := NewSession("", badpem, 0, "wah", nullog)
	c.Check(sess, IsNil)
	c.Check(err, NotNil)
}

/****************************************************************
  Dial() tests
****************************************************************/

func (cs *clientSessionSuite) TestDialFailsWithNoAddress(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	err = sess.Dial()
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*dial.*address.*")
}

func (cs *clientSessionSuite) TestDialConnects(c *C) {
	srv, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, IsNil)
	defer srv.Close()
	sess, err := NewSession(srv.Addr().String(), nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	err = sess.Dial()
	c.Check(err, IsNil)
	c.Check(sess.Connection, NotNil)
}

func (cs *clientSessionSuite) TestDialConnectFail(c *C) {
	srv, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, IsNil)
	sess, err := NewSession(srv.Addr().String(), nil, 0, "wah", debuglog)
	srv.Close()
	c.Assert(err, IsNil)
	err = sess.Dial()
	c.Check(sess.Connection, IsNil)
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*connection refused")
}

/****************************************************************
  Close() tests
****************************************************************/

func (cs *clientSessionSuite) TestClose(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestClose"}
	sess.Close()
	c.Check(sess.Connection, IsNil)
}

func (cs *clientSessionSuite) TestCloseTwice(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestCloseTwice"}
	sess.Close()
	c.Check(sess.Connection, IsNil)
	sess.Close()
	c.Check(sess.Connection, IsNil)
}

func (cs *clientSessionSuite) TestCloseFails(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestCloseFails", CloseCondition: condition.Work(false)}
	sess.Close()
	c.Check(sess.Connection, IsNil) // nothing you can do to clean up anyway
}

/****************************************************************
  checkRunnable() tests
****************************************************************/

func (cs *clientSessionSuite) TestCheckRunnableFailsIfNoConnection(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	// no connection!
	c.Check(sess.checkRunnable(), NotNil)
}

func (cs *clientSessionSuite) TestCheckRunnableFailsIfNoProtocolator(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	// set up the connection
	sess.Connection = &testConn{}
	// And stomp on the protocolator
	sess.Protocolator = nil
	c.Check(sess.checkRunnable(), NotNil)
}

func (cs *clientSessionSuite) TestCheckRunnable(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	// set up the connection
	sess.Connection = &testConn{}
	c.Check(sess.checkRunnable(), IsNil)
}

/****************************************************************
  handlePing() tests
****************************************************************/

type msgSuite struct {
	sess   *ClientSession
	upCh   chan interface{}
	downCh chan interface{}
	errCh  chan error
}

var _ = Suite(&msgSuite{})

func (s *msgSuite) SetUpTest(c *C) {
	var err error
	s.sess, err = NewSession("", nil, time.Millisecond, "wah", debuglog)
	c.Assert(err, IsNil)
	s.sess.Connection = &testConn{Name: "TestHandle*"}
	s.errCh = make(chan error, 1)
	s.upCh = make(chan interface{}, 5)
	s.downCh = make(chan interface{}, 5)
	s.sess.proto = &testProtocol{up: s.upCh, down: s.downCh}
	// make the message channel buffered
	s.sess.MsgCh = make(chan *Notification, 5)
}

func (s *msgSuite) TestHandlePingWorks(c *C) {
	s.upCh <- nil // no error
	c.Check(s.sess.handlePing(), IsNil)
	c.Assert(len(s.downCh), Equals, 2)
	c.Check(<-s.downCh, Equals, "deadline 1ms")
	c.Check(<-s.downCh, Equals, protocol.PingPongMsg{Type: "pong"})
}

func (s *msgSuite) TestHandlePingHandlesPongWriteError(c *C) {
	failure := errors.New("Pong")
	s.upCh <- failure

	c.Check(s.sess.handlePing(), Equals, failure)
	c.Assert(len(s.downCh), Equals, 2)
	c.Check(<-s.downCh, Equals, "deadline 1ms")
	c.Check(<-s.downCh, Equals, protocol.PingPongMsg{Type: "pong"})
}

/****************************************************************
  handleBroadcast() tests
****************************************************************/

func (s *msgSuite) TestHandleBroadcastWorks(c *C) {
	msg := serverMsg{"broadcast",
		protocol.BroadcastMsg{
			Type:     "broadcast",
			AppId:    "--ignored--",
			ChanId:   "0",
			TopLevel: 2,
			Payloads: []json.RawMessage{json.RawMessage(`{"b":1}`)},
		}, protocol.NotificationsMsg{}}
	go func() { s.errCh <- s.sess.handleBroadcast(&msg) }()
	c.Check(takeNext(s.downCh), Equals, "deadline 1ms")
	c.Check(takeNext(s.downCh), Equals, protocol.PingPongMsg{Type: "ack"})
	s.upCh <- nil // ack ok
	c.Check(<-s.errCh, Equals, nil)
	c.Assert(len(s.sess.MsgCh), Equals, 1)
	c.Check(<-s.sess.MsgCh, Equals, &Notification{})
	// and finally, the session keeps track of the levels
	c.Check(s.sess.Levels.GetAll(), DeepEquals, map[string]int64{"0": 2})
}

func (s *msgSuite) TestHandleBroadcastBadAckWrite(c *C) {
	msg := serverMsg{"broadcast",
		protocol.BroadcastMsg{
			Type:     "broadcast",
			AppId:    "APP",
			ChanId:   "0",
			TopLevel: 2,
			Payloads: []json.RawMessage{json.RawMessage(`{"b":1}`)},
		}, protocol.NotificationsMsg{}}
	go func() { s.errCh <- s.sess.handleBroadcast(&msg) }()
	c.Check(takeNext(s.downCh), Equals, "deadline 1ms")
	c.Check(takeNext(s.downCh), Equals, protocol.PingPongMsg{Type: "ack"})
	failure := errors.New("ACK ACK ACK")
	s.upCh <- failure
	c.Assert(<-s.errCh, Equals, failure)
}

func (s *msgSuite) TestHandleBroadcastWrongChannel(c *C) {
	msg := serverMsg{"broadcast",
		protocol.BroadcastMsg{
			Type:     "broadcast",
			AppId:    "APP",
			ChanId:   "something awful",
			TopLevel: 2,
			Payloads: []json.RawMessage{json.RawMessage(`{"b":1}`)},
		}, protocol.NotificationsMsg{}}
	go func() { s.errCh <- s.sess.handleBroadcast(&msg) }()
	c.Check(takeNext(s.downCh), Equals, "deadline 1ms")
	c.Check(takeNext(s.downCh), Equals, protocol.PingPongMsg{Type: "ack"})
	s.upCh <- nil // ack ok
	c.Check(<-s.errCh, IsNil)
	c.Check(len(s.sess.MsgCh), Equals, 0)
}
