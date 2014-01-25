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
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSession(t *testing.T) { TestingT(t) }

type clientSessionSuite struct{}

var nullog = logger.NewSimpleLogger(ioutil.Discard, "error")
var debuglog = logger.NewSimpleLogger(os.Stderr, "debug")
var _ = Suite(&clientSessionSuite{})

/****************************************************************
  NewSession() tests
****************************************************************/

func (cs *clientSessionSuite) TestNewSessionPlainWorks(c *C) {
	cfg := Config{}
	sess, err := NewSession(cfg, nullog, "wah")
	c.Check(sess, NotNil)
	c.Check(err, IsNil)
}

var certfile string = helpers.SourceRelative("../../server/acceptance/config/testing.cert")

func (cs *clientSessionSuite) TestNewSessionPEMWorks(c *C) {
	cfg := Config{CertPEMFile: certfile}
	sess, err := NewSession(cfg, nullog, "wah")
	c.Check(sess, NotNil)
	c.Assert(err, IsNil)
	c.Check(sess.TLS.RootCAs, NotNil)
}

func (cs *clientSessionSuite) TestNewSessionBadPEMFilePathFails(c *C) {
	cfg := Config{CertPEMFile: "/no/such/path"}
	sess, err := NewSession(cfg, nullog, "wah")
	c.Check(sess, IsNil)
	c.Check(err, NotNil)
}

func (cs *clientSessionSuite) TestNewSessionBadPEMFileContentFails(c *C) {
	cfg := Config{CertPEMFile: "/etc/passwd"}
	sess, err := NewSession(cfg, nullog, "wah")
	c.Check(sess, IsNil)
	c.Check(err, NotNil)
}

/****************************************************************
  Run() tests
****************************************************************/

func testname() string {
	pcs := make([]uintptr, 200)
	runtime.Callers(0, pcs)
	testname := "<unknown>"
	for _, pc := range pcs {
		me := runtime.FuncForPC(pc)
		if me == nil {
			break
		}
		parts := strings.Split(me.Name(), ".")
		funcname := parts[len(parts)-1]
		if strings.HasPrefix(funcname, "Test") {
			testname = funcname
		}
	}
	return testname
}

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
}

func (tc *testConn) LocalAddr() net.Addr { return xAddr(tc.Name) }

func (tc *testConn) RemoteAddr() net.Addr { return xAddr(tc.Name) }

func (tc *testConn) Close() error { return nil }

func (tc *testConn) SetDeadline(t time.Time) error {
	tc.Deadlines = append(tc.Deadlines, t.Sub(time.Now()))
	if tc.DeadlineCondition == nil || tc.DeadlineCondition.OK() {
		return nil
	} else {
		return errors.New("deadliner on fire")
	}
}

func (tc *testConn) SetReadDeadline(t time.Time) error  { panic("NIH"); return nil }
func (tc *testConn) SetWriteDeadline(t time.Time) error { panic("NIH"); return nil }
func (tc *testConn) Read(buf []byte) (n int, err error) { panic("NIH"); return -1, nil }

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
 *
 *  Go way down to the bottom if you want to see the full, working case.
 *  This has a rather slow buildup.
 *
 *  TODO: check deadlines
 *
 ****************************************************************/

func (cs *clientSessionSuite) TestRunFailsIfNilConnection(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	// not connected!
	err = sess.run()
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*disconnected.*")
}

func (cs *clientSessionSuite) TestRunFailsIfNilProtocolator(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname()} // ok, have a constructor
	sess.Protocolator = nil                       // but no protocol, seeficare.
	err = sess.run()
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*protocol constructor.*")
}

func (cs *clientSessionSuite) TestRunFailsIfSetDeadlineFails(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname(),
		DeadlineCondition: condition.Work(false)} // setdeadline will fail
	err = sess.run()
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*deadline.*")
}

func (cs *clientSessionSuite) TestRunFailsIfWriteFails(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname(),
		WriteCondition: condition.Work(false)} // write will fail
	err = sess.run()
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*write.*")
}

func (cs *clientSessionSuite) TestRunConnectMessageFails(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname()}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	go func() {
		errCh <- sess.run()
	}()

	c.Check(takeNext(downCh), DeepEquals, protocol.ConnectMsg{
		Type:     "connect",
		DeviceId: sess.DeviceId,
		Levels:   map[string]int64{},
	})
	upCh <- errors.New("Overflow error in /dev/null")
	err = <-errCh
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, "Overflow.*null")
}

func (cs *clientSessionSuite) TestRunConnackReadError(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname()}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	go func() {
		errCh <- sess.run()
	}()

	takeNext(downCh) // connectMsg
	upCh <- nil      // no error
	upCh <- io.EOF
	err = <-errCh
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*EOF.*")
}

func (cs *clientSessionSuite) TestRunBadConnack(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname()}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	go func() {
		errCh <- sess.run()
	}()

	takeNext(downCh) // connectMsg
	upCh <- nil      // no error
	upCh <- protocol.ConnAckMsg{}
	err = <-errCh
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*invalid.*")
}

func (cs *clientSessionSuite) TestRunMainloopReadError(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname()}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	go func() {
		errCh <- sess.run()
	}()

	takeNext(downCh) // connectMsg
	upCh <- nil      // no error
	upCh <- protocol.ConnAckMsg{
		Type:   "connack",
		Params: protocol.ConnAckParams{(10 * time.Millisecond).String()},
	}
	// in the mainloop!
	upCh <- errors.New("Read")
	err = <-errCh
	c.Assert(err, NotNil)
	c.Check(err.Error(), Equals, "Read")
}

func (cs *clientSessionSuite) TestRunPongWriteError(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname()}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	go func() {
		errCh <- sess.run()
	}()

	takeNext(downCh) // connectMsg
	upCh <- nil      // no error
	upCh <- protocol.ConnAckMsg{
		Type:   "connack",
		Params: protocol.ConnAckParams{(10 * time.Millisecond).String()},
	}
	// in the mainloop!
	upCh <- protocol.PingPongMsg{Type: "ping"}
	c.Check(takeNext(downCh), Equals, protocol.PingPongMsg{Type: "pong"})
	upCh <- errors.New("Pong")
	err = <-errCh
	c.Assert(err, NotNil)
	c.Check(err.Error(), Equals, "Pong")
}

func (cs *clientSessionSuite) TestRunPingPong(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname()}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	go func() {
		errCh <- sess.run()
	}()

	takeNext(downCh) // connectMsg
	upCh <- nil      // no error
	upCh <- protocol.ConnAckMsg{
		Type:   "connack",
		Params: protocol.ConnAckParams{(10 * time.Millisecond).String()},
	}
	// in the mainloop!
	upCh <- protocol.PingPongMsg{Type: "ping"}
	c.Check(takeNext(downCh), Equals, protocol.PingPongMsg{Type: "pong"})
	upCh <- nil    // pong ok
	upCh <- io.EOF // close it down
	err = <-errCh
}

func (cs *clientSessionSuite) TestRunBadAckWrite(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname()}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }
	sess.MsgCh = make(chan *Notification, 5)

	go func() {
		errCh <- sess.run()
	}()

	takeNext(downCh) // connectMsg
	upCh <- nil      // no error
	upCh <- protocol.ConnAckMsg{
		Type:   "connack",
		Params: protocol.ConnAckParams{time.Second.String()},
	}
	// in the mainloop!

	b := &protocol.BroadcastMsg{
		Type:     "broadcast",
		AppId:    "APP",
		ChanId:   "0",
		TopLevel: 2,
		Payloads: []json.RawMessage{json.RawMessage(`{"b":1}`)},
	}
	upCh <- b
	c.Check(takeNext(downCh), Equals, protocol.PingPongMsg{Type: "ack"})
	upCh <- errors.New("ACK ACK ACK")
	err = <-errCh
	c.Assert(err, NotNil)
	c.Check(err.Error(), Equals, "ACK ACK ACK")
}

func (cs *clientSessionSuite) TestRunBroadcastWrongChannel(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname()}
	errCh := make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }
	sess.MsgCh = make(chan *Notification, 5)

	go func() {
		errCh <- sess.run()
	}()

	takeNext(downCh) // connectMsg
	upCh <- nil      // no error
	upCh <- protocol.ConnAckMsg{
		Type:   "connack",
		Params: protocol.ConnAckParams{time.Second.String()},
	}
	// in the mainloop!

	b := &protocol.BroadcastMsg{
		Type:     "broadcast",
		AppId:    "APP",
		ChanId:   "42",
		TopLevel: 2,
		Payloads: []json.RawMessage{json.RawMessage(`{"b":1}`)},
	}
	upCh <- b
	c.Check(takeNext(downCh), Equals, protocol.PingPongMsg{Type: "ack"})
	upCh <- nil    // ack ok
	upCh <- io.EOF // close it down
	err = <-errCh
	c.Check(len(sess.MsgCh), Equals, 0)
}

func (cs *clientSessionSuite) TestRunBroadcastRightChannel(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: testname()}
	sess.ErrCh = make(chan error, 1)
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }
	sess.MsgCh = make(chan *Notification, 5)

	sess.Run()

	takeNext(downCh) // connectMsg
	upCh <- nil      // no error
	upCh <- protocol.ConnAckMsg{
		Type:   "connack",
		Params: protocol.ConnAckParams{time.Second.String()},
	}
	// in the mainloop!

	b := &protocol.BroadcastMsg{
		Type:     "broadcast",
		AppId:    "--ignored--",
		ChanId:   "0",
		TopLevel: 2,
		Payloads: []json.RawMessage{json.RawMessage(`{"b":1}`)},
	}
	upCh <- b
	c.Check(takeNext(downCh), Equals, protocol.PingPongMsg{Type: "ack"})
	upCh <- nil    // ack ok
	upCh <- io.EOF // close it down
	err = <-sess.ErrCh
	c.Assert(len(sess.MsgCh), Equals, 1)
	c.Check(<-sess.MsgCh, Equals, &Notification{})
	// and finally, the session keeps track of the levels
	c.Check(sess.Levels.GetAll(), DeepEquals, map[string]int64{"0": 2})
}

/*
 *
 *
 *
 * breathe in...
 */

func (cs *clientSessionSuite) TestDialFailsWithNoAddress(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	err = sess.Dial()
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*dial.*address.*")
}

func (cs *clientSessionSuite) TestDialConnects(c *C) {
	lp, err := net.Listen("tcp", ":0")
	c.Assert(err, IsNil)
	defer lp.Close()
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.ServerAddr = lp.Addr().String()
	err = sess.Dial()
	c.Check(err, IsNil)
	c.Check(sess.Connection, NotNil)
}

func (cs *clientSessionSuite) TestResetFailsWithoutProtocolator(c *C) {
	sess, _ := NewSession(Config{}, debuglog, "wah")
	sess.Protocolator = nil
	err := sess.Reset()
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*protocol constructor\\.")
}

func (cs *clientSessionSuite) TestResetFailsWithNoAddress(c *C) {
	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	err = sess.Reset()
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*dial.*address.*")
}

func (cs *clientSessionSuite) TestResets(c *C) {
	upCh := make(chan interface{}, 5)
	downCh := make(chan interface{}, 5)
	proto := &testProtocol{up: upCh, down: downCh}
	lp, err := net.Listen("tcp", ":0")
	c.Assert(err, IsNil)
	defer lp.Close()

	sess, err := NewSession(Config{}, debuglog, "wah")
	c.Assert(err, IsNil)
	sess.ServerAddr = lp.Addr().String()
	sess.Connection = &testConn{Name: testname()}
	sess.Protocolator = func(_ net.Conn) protocol.Protocol { return proto }

	sess.Reset()

	// wheee
	err = <-sess.ErrCh
	c.Assert(err, NotNil) // some random tcp error because
	// there's nobody talking to the port
}
