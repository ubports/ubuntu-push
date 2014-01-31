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
	"io"
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

func (tc *testConn) SetDeadline(t time.Time) error {
	tc.Deadlines = append(tc.Deadlines, t.Sub(time.Now()))
	if tc.DeadlineCondition == nil || tc.DeadlineCondition.OK() {
		return nil
	} else {
		return errors.New("deadliner on fire")
	}
}

func (tc *testConn) SetReadDeadline(t time.Time) error  { panic("SetReadDeadline not implemented.") }
func (tc *testConn) SetWriteDeadline(t time.Time) error { panic("SetWriteDeadline not implemented.") }
func (tc *testConn) Read(buf []byte) (n int, err error) { panic("Read not implemented.") }

func (tc *testConn) Write(buf []byte) (int, error) {
	store := make([]byte, len(buf))
	copy(store, buf)
	tc.Writes = append(tc.Writes, store)
	if tc.WriteCondition == nil || tc.WriteCondition.OK() {
		return len(store), nil
	} else {
		return -1, errors.New("writer on fire")
	}
}

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
	switch v := takeNext(c.up).(type) {
	case error:
		return v
	default:
		// make sure JSON.Unmarshal works with dest
		var marshalledMsg []byte
		marshalledMsg, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("can't jsonify test value %v: %s", v, err)
		}
		return json.Unmarshal(marshalledMsg, dest)
	}
	return nil
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
  connect() tests
****************************************************************/

func (cs *clientSessionSuite) TestConnectFailsWithNoAddress(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	err = sess.connect()
	c.Check(err, ErrorMatches, ".*connect.*address.*")
}

func (cs *clientSessionSuite) TestConnectConnects(c *C) {
	srv, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, IsNil)
	defer srv.Close()
	sess, err := NewSession(srv.Addr().String(), nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	err = sess.connect()
	c.Check(err, IsNil)
	c.Check(sess.Connection, NotNil)
}

func (cs *clientSessionSuite) TestConnectConnectFail(c *C) {
	srv, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, IsNil)
	sess, err := NewSession(srv.Addr().String(), nil, 0, "wah", debuglog)
	srv.Close()
	c.Assert(err, IsNil)
	err = sess.connect()
	c.Check(err, ErrorMatches, ".*connection refused")
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
	c.Assert(len(s.downCh), Equals, 1)
	c.Check(<-s.downCh, Equals, protocol.PingPongMsg{Type: "pong"})
}

func (s *msgSuite) TestHandlePingHandlesPongWriteError(c *C) {
	failure := errors.New("Pong")
	s.upCh <- failure

	c.Check(s.sess.handlePing(), Equals, failure)
	c.Assert(len(s.downCh), Equals, 1)
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
	c.Check(takeNext(s.downCh), Equals, protocol.AckMsg{"ack"})
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
	c.Check(takeNext(s.downCh), Equals, protocol.AckMsg{"ack"})
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
	c.Check(takeNext(s.downCh), Equals, protocol.AckMsg{"ack"})
	s.upCh <- nil // ack ok
	c.Check(<-s.errCh, IsNil)
	c.Check(len(s.sess.MsgCh), Equals, 0)
}

/****************************************************************
  loop() tests
****************************************************************/

type loopSuite msgSuite

var _ = Suite(&loopSuite{})

func (s *loopSuite) SetUpTest(c *C) {
	(*msgSuite)(s).SetUpTest(c)
	s.sess.Connection.(*testConn).Name = "TestLoop*"
	go func() {
		s.errCh <- s.sess.loop()
	}()
}

func (s *loopSuite) TestLoopReadError(c *C) {
	s.upCh <- errors.New("Read")
	err := <-s.errCh
	c.Check(err, ErrorMatches, "Read")
}

func (s *loopSuite) TestLoopPing(c *C) {
	c.Check(takeNext(s.downCh), Equals, "deadline 1ms")
	s.upCh <- protocol.PingPongMsg{Type: "ping"}
	c.Check(takeNext(s.downCh), Equals, protocol.PingPongMsg{Type: "pong"})
	failure := errors.New("pong")
	s.upCh <- failure
	c.Check(<-s.errCh, Equals, failure)
}

func (s *loopSuite) TestLoopLoopsDaLoop(c *C) {
	for i := 1; i < 10; i++ {
		c.Check(takeNext(s.downCh), Equals, "deadline 1ms")
		s.upCh <- protocol.PingPongMsg{Type: "ping"}
		c.Check(takeNext(s.downCh), Equals, protocol.PingPongMsg{Type: "pong"})
		s.upCh <- nil
	}
	failure := errors.New("pong")
	s.upCh <- failure
	c.Check(<-s.errCh, Equals, failure)
}

func (s *loopSuite) TestLoopBroadcast(c *C) {
	b := &protocol.BroadcastMsg{
		Type:     "broadcast",
		AppId:    "--ignored--",
		ChanId:   "0",
		TopLevel: 2,
		Payloads: []json.RawMessage{json.RawMessage(`{"b":1}`)},
	}
	c.Check(takeNext(s.downCh), Equals, "deadline 1ms")
	s.upCh <- b
	c.Check(takeNext(s.downCh), Equals, protocol.AckMsg{"ack"})
	failure := errors.New("ack")
	s.upCh <- failure
	c.Check(<-s.errCh, Equals, failure)
}

/****************************************************************
  start() tests
****************************************************************/
func (cs *clientSessionSuite) TestStartFailsIfSetDeadlineFails(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestStartFailsIfSetDeadlineFails",
		DeadlineCondition: condition.Work(false)} // setdeadline will fail
	err = sess.start()
	c.Check(err, ErrorMatches, ".*deadline.*")
}

func (cs *clientSessionSuite) TestStartFailsIfWriteFails(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestStartFailsIfWriteFails",
		WriteCondition: condition.Work(false)} // write will fail
	err = sess.start()
	c.Check(err, ErrorMatches, ".*write.*")
}

func (cs *clientSessionSuite) TestStartConnectMessageFails(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestStartConnectMessageFails"}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	go func() {
		errCh <- sess.start()
	}()

	c.Check(takeNext(downCh), Equals, "deadline 0")
	c.Check(takeNext(downCh), DeepEquals, protocol.ConnectMsg{
		Type:     "connect",
		DeviceId: sess.DeviceId,
		Levels:   map[string]int64{},
	})
	upCh <- errors.New("Overflow error in /dev/null")
	err = <-errCh
	c.Check(err, ErrorMatches, "Overflow.*null")
}

func (cs *clientSessionSuite) TestStartConnackReadError(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestStartConnackReadError"}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	go func() {
		errCh <- sess.start()
	}()

	c.Check(takeNext(downCh), Equals, "deadline 0")
	_, ok := takeNext(downCh).(protocol.ConnectMsg)
	c.Check(ok, Equals, true)
	upCh <- nil // no error
	upCh <- io.EOF
	err = <-errCh
	c.Check(err, ErrorMatches, ".*EOF.*")
}

func (cs *clientSessionSuite) TestStartBadConnack(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestStartBadConnack"}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	go func() {
		errCh <- sess.start()
	}()

	c.Check(takeNext(downCh), Equals, "deadline 0")
	_, ok := takeNext(downCh).(protocol.ConnectMsg)
	c.Check(ok, Equals, true)
	upCh <- nil // no error
	upCh <- protocol.ConnAckMsg{Type: "connack"}
	err = <-errCh
	c.Check(err, ErrorMatches, ".*invalid.*")
}

func (cs *clientSessionSuite) TestStartNotConnack(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestStartBadConnack"}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	go func() {
		errCh <- sess.start()
	}()

	c.Check(takeNext(downCh), Equals, "deadline 0")
	_, ok := takeNext(downCh).(protocol.ConnectMsg)
	c.Check(ok, Equals, true)
	upCh <- nil // no error
	upCh <- protocol.ConnAckMsg{Type: "connnak"}
	err = <-errCh
	c.Check(err, ErrorMatches, ".*CONNACK.*")
}

func (cs *clientSessionSuite) TestStartWorks(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestStartWorks"}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	go func() {
		errCh <- sess.start()
	}()

	c.Check(takeNext(downCh), Equals, "deadline 0")
	_, ok := takeNext(downCh).(protocol.ConnectMsg)
	c.Check(ok, Equals, true)
	upCh <- nil // no error
	upCh <- protocol.ConnAckMsg{
		Type:   "connack",
		Params: protocol.ConnAckParams{(10 * time.Millisecond).String()},
	}
	// start is now done.
	err = <-errCh
	c.Assert(err, IsNil)
}
