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
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/broker"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestSession(t *testing.T) { TestingT(t) }

type sessionSuite struct{}

var _ = Suite(&sessionSuite{})

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
			return fmt.Errorf("can't jsonify test value: %v", v)
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

type testSessionConfig struct {
	exchangeTimeout time.Duration
	pingInterval    time.Duration
}

func (tsc *testSessionConfig) PingInterval() time.Duration {
	return tsc.pingInterval
}

func (tsc *testSessionConfig) ExchangeTimeout() time.Duration {
	return tsc.exchangeTimeout
}

var cfg5msExchangeTout = &testSessionConfig{
	exchangeTimeout: 5 * time.Millisecond,
}

type testBroker struct {
	registration chan interface{}
	err          error
}

func newTestBroker() *testBroker {
	return &testBroker{registration: make(chan interface{}, 2)}
}

type testBrokerSession struct {
	deviceId  string
	exchanges chan broker.Exchange
}

func (tbs *testBrokerSession) SessionChannel() <-chan broker.Exchange {
	return tbs.exchanges
}

func (tbs *testBrokerSession) DeviceId() string {
	return tbs.deviceId
}

func (tb *testBroker) Register(connect *protocol.ConnectMsg) (broker.BrokerSession, error) {
	tb.registration <- "register " + connect.DeviceId
	return &testBrokerSession{connect.DeviceId, nil}, tb.err
}

func (tb *testBroker) Unregister(sess broker.BrokerSession) {
	tb.registration <- "unregister " + sess.(*testBrokerSession).deviceId
}

func (s *sessionSuite) TestSessionStart(c *C) {
	var sess broker.BrokerSession
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	brkr := newTestBroker()
	go func() {
		var err error
		sess, err = sessionStart(tp, brkr, cfg5msExchangeTout)
		errCh <- err
	}()
	c.Check(takeNext(down), Equals, "deadline 5ms")
	up <- protocol.ConnectMsg{Type: "connect", ClientVer: "1", DeviceId: "dev-1"}
	err := <-errCh
	c.Check(err, IsNil)
	c.Check(takeNext(brkr.registration), Equals, "register dev-1")
	c.Check(sess.(*testBrokerSession).deviceId, Equals, "dev-1")
}

func (s *sessionSuite) TestSessionRegisterError(c *C) {
	var sess broker.BrokerSession
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	brkr := newTestBroker()
	errRegister := errors.New("register failure")
	brkr.err = errRegister
	go func() {
		var err error
		sess, err = sessionStart(tp, brkr, cfg5msExchangeTout)
		errCh <- err
	}()
	up <- protocol.ConnectMsg{Type: "connect", ClientVer: "1", DeviceId: "dev-1"}
	err := <-errCh
	c.Check(err, Equals, errRegister)
}

func (s *sessionSuite) TestSessionStartErrors(c *C) {
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	up <- io.ErrUnexpectedEOF
	_, err := sessionStart(tp, nil, cfg5msExchangeTout)
	c.Check(err, Equals, io.ErrUnexpectedEOF)
}

func (s *sessionSuite) TestSessionStartMismatch(c *C) {
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	up <- protocol.ConnectMsg{Type: "what"}
	_, err := sessionStart(tp, nil, cfg5msExchangeTout)
	c.Check(err, DeepEquals, &broker.ErrAbort{"expected CONNECT message"})
}

var cfg5msPingInterval2msExchangeTout = &testSessionConfig{
	pingInterval:    5 * time.Millisecond,
	exchangeTimeout: 2 * time.Millisecond,
}

func (s *sessionSuite) TestSessionLoop(c *C) {
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	sess := &testBrokerSession{}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, protocol.PingPongMsg{Type: "ping"})
	up <- nil // no write error
	up <- protocol.PingPongMsg{Type: "pong"}
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, protocol.PingPongMsg{Type: "ping"})
	up <- nil // no write error
	up <- io.ErrUnexpectedEOF
	err := <-errCh
	c.Check(err, Equals, io.ErrUnexpectedEOF)
}

func (s *sessionSuite) TestSessionLoopWriteError(c *C) {
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	sess := &testBrokerSession{}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), FitsTypeOf, protocol.PingPongMsg{})
	up <- io.ErrUnexpectedEOF // write error
	err := <-errCh
	c.Check(err, Equals, io.ErrUnexpectedEOF)
}

func (s *sessionSuite) TestSessionLoopMismatch(c *C) {
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	sess := &testBrokerSession{}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, protocol.PingPongMsg{Type: "ping"})
	up <- nil // no write error
	up <- protocol.PingPongMsg{Type: "what"}
	err := <-errCh
	c.Check(err, DeepEquals, &broker.ErrAbort{"expected PONG message"})
}

type testMsg struct {
	Type   string `json:"T"`
	Part   int    `json:"P"`
	nParts int
}

func (m *testMsg) Split() bool {
	if m.nParts == 0 {
		return true
	}
	m.Part++
	if m.Part == m.nParts {
		return true
	}
	return false
}

type testExchange struct {
	inMsg    testMsg
	prepErr  error
	finErr   error
	finSleep time.Duration
	nParts   int
}

func (exchg *testExchange) Prepare(sess broker.BrokerSession) (outMsg protocol.SplittableMsg, inMsg interface{}, err error) {
	return &testMsg{Type: "msg", nParts: exchg.nParts}, &exchg.inMsg, exchg.prepErr
}

func (exchg *testExchange) Acked(sess broker.BrokerSession) error {
	time.Sleep(exchg.finSleep)
	return exchg.finErr
}

func (s *sessionSuite) TestSessionLoopExchange(c *C) {
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	exchanges <- &testExchange{}
	sess := &testBrokerSession{exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, testMsg{Type: "msg"})
	up <- nil // no write error
	up <- testMsg{Type: "ack"}
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, protocol.PingPongMsg{Type: "ping"})
	up <- nil // no write error
	up <- io.EOF
	err := <-errCh
	c.Check(err, Equals, io.EOF)
}

func (s *sessionSuite) TestSessionLoopExchangeSplit(c *C) {
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	exchanges <- &testExchange{nParts: 2}
	sess := &testBrokerSession{exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, testMsg{Type: "msg", Part: 1, nParts: 2})
	up <- nil // no write error
	up <- testMsg{Type: "ack"}
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, testMsg{Type: "msg", Part: 2, nParts: 2})
	up <- nil // no write error
	up <- testMsg{Type: "ack"}
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, protocol.PingPongMsg{Type: "ping"})
	up <- nil // no write error
	up <- io.EOF
	err := <-errCh
	c.Check(err, Equals, io.EOF)
}

func (s *sessionSuite) TestSessionLoopExchangePrepareError(c *C) {
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	prepErr := errors.New("prepare failure")
	exchanges <- &testExchange{prepErr: prepErr}
	sess := &testBrokerSession{exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout)
	}()
	err := <-errCh
	c.Check(err, Equals, prepErr)
}

func (s *sessionSuite) TestSessionLoopExchangeAckedError(c *C) {
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	finErr := errors.New("finish error")
	exchanges <- &testExchange{finErr: finErr}
	sess := &testBrokerSession{exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, testMsg{Type: "msg"})
	up <- nil // no write error
	up <- testMsg{Type: "ack"}
	err := <-errCh
	c.Check(err, Equals, finErr)
}

func (s *sessionSuite) TestSessionLoopExchangeWriteError(c *C) {
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	exchanges <- &testExchange{}
	sess := &testBrokerSession{exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), FitsTypeOf, testMsg{})
	up <- io.ErrUnexpectedEOF
	err := <-errCh
	c.Check(err, Equals, io.ErrUnexpectedEOF)
}

func (s *sessionSuite) TestSessionLoopExchangeNextPing(c *C) {
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	exchanges <- &testExchange{finSleep: 3 * time.Millisecond}
	sess := &testBrokerSession{exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, testMsg{Type: "msg"})
	up <- nil // no write error
	up <- testMsg{Type: "ack"}
	tack := time.Now() // next ping interval starts around here
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, protocol.PingPongMsg{Type: "ping"})
	c.Check(time.Since(tack) < (3+5)*time.Millisecond, Equals, true)
	up <- nil // no write error
	up <- io.EOF
	err := <-errCh
	c.Check(err, Equals, io.EOF)
}

func serverClientWire() (srv net.Conn, cli net.Conn, lst net.Listener) {
	lst, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	cli, err = net.DialTCP("tcp", nil, lst.Addr().(*net.TCPAddr))
	if err != nil {
		panic(err)
	}
	srv, err = lst.Accept()
	if err != nil {
		panic(err)
	}
	return
}

type rememberDeadlineConn struct {
	net.Conn
	deadlineKind []string
}

func (c *rememberDeadlineConn) SetDeadline(t time.Time) error {
	c.deadlineKind = append(c.deadlineKind, "both")
	return c.Conn.SetDeadline(t)
}

func (c *rememberDeadlineConn) SetReadDeadline(t time.Time) error {
	c.deadlineKind = append(c.deadlineKind, "read")
	return c.Conn.SetDeadline(t)
}

func (c *rememberDeadlineConn) SetWriteDeadline(t time.Time) error {
	c.deadlineKind = append(c.deadlineKind, "write")
	return c.Conn.SetDeadline(t)
}

var cfg5msPingInterval = &testSessionConfig{
	pingInterval:    5 * time.Millisecond,
	exchangeTimeout: 5 * time.Millisecond,
}

var nopLogger = logger.NewSimpleLogger(ioutil.Discard, "error")

func (s *sessionSuite) TestSessionWire(c *C) {
	buf := &bytes.Buffer{}
	track := NewTracker(logger.NewSimpleLogger(buf, "debug"))
	errCh := make(chan error, 1)
	srv, cli, lst := serverClientWire()
	defer lst.Close()
	remSrv := &rememberDeadlineConn{srv, make([]string, 0, 2)}
	brkr := newTestBroker()
	go func() {
		errCh <- Session(remSrv, brkr, cfg5msPingInterval, track)
	}()
	io.WriteString(cli, "\x00")
	io.WriteString(cli, "\x00\x20{\"T\":\"connect\",\"DeviceId\":\"DEV\"}")
	downStream := bufio.NewReader(cli)
	msg, err := downStream.ReadBytes(byte('}'))
	c.Check(err, IsNil)
	c.Check(msg, DeepEquals, []byte("\x00\x0c{\"T\":\"ping\"}"))
	c.Check(takeNext(brkr.registration), Equals, "register DEV")
	c.Check(len(brkr.registration), Equals, 0) // not yet unregistered
	cli.Close()
	err = <-errCh
	c.Check(remSrv.deadlineKind, DeepEquals, []string{"read", "both", "both"})
	c.Check(err, Equals, io.EOF)
	c.Check(takeNext(brkr.registration), Equals, "unregister DEV")
	// tracking
	c.Check(buf.String(), Matches, `.*connected.*\n.*registered DEV.*\n.*ended with: EOF\n`)
}

func (s *sessionSuite) TestSessionWireTimeout(c *C) {
	nopTrack := NewTracker(nopLogger)
	errCh := make(chan error, 1)
	srv, cli, lst := serverClientWire()
	defer lst.Close()
	remSrv := &rememberDeadlineConn{srv, make([]string, 0, 2)}
	brkr := newTestBroker()
	go func() {
		errCh <- Session(remSrv, brkr, cfg5msPingInterval2msExchangeTout, nopTrack)
	}()
	err := <-errCh
	c.Check(err, FitsTypeOf, &net.OpError{})
	c.Check(remSrv.deadlineKind, DeepEquals, []string{"read"})
	cli.Close()
}

func (s *sessionSuite) TestSessionWireWrongVersion(c *C) {
	buf := &bytes.Buffer{}
	track := NewTracker(logger.NewSimpleLogger(buf, "debug"))
	errCh := make(chan error, 1)
	srv, cli, lst := serverClientWire()
	defer lst.Close()
	brkr := newTestBroker()
	go func() {
		errCh <- Session(srv, brkr, cfg5msPingInterval, track)
	}()
	io.WriteString(cli, "\x10")
	err := <-errCh
	c.Check(err, DeepEquals, &broker.ErrAbort{"unexpected wire format version"})
	cli.Close()
	// tracking
	c.Check(buf.String(), Matches, `.*connected.*\n.*ended with: session aborted \(unexpected.*version\)\n`)

}

func (s *sessionSuite) TestSessionWireEarlyClose(c *C) {
	buf := &bytes.Buffer{}
	track := NewTracker(logger.NewSimpleLogger(buf, "debug"))
	errCh := make(chan error, 1)
	srv, cli, lst := serverClientWire()
	defer lst.Close()
	brkr := newTestBroker()
	go func() {
		errCh <- Session(srv, brkr, cfg5msPingInterval, track)
	}()
	cli.Close()
	err := <-errCh
	c.Check(err, Equals, io.EOF)
	// tracking
	c.Check(buf.String(), Matches, `.*connected.*\n.*ended with: EOF\n`)

}

func (s *sessionSuite) TestSessionWireEarlyClose2(c *C) {
	buf := &bytes.Buffer{}
	track := NewTracker(logger.NewSimpleLogger(buf, "debug"))
	errCh := make(chan error, 1)
	srv, cli, lst := serverClientWire()
	defer lst.Close()
	brkr := newTestBroker()
	go func() {
		errCh <- Session(srv, brkr, cfg5msPingInterval, track)
	}()
	io.WriteString(cli, "\x00")
	io.WriteString(cli, "\x00")
	cli.Close()
	err := <-errCh
	c.Check(err, Equals, io.EOF)
	// tracking
	c.Check(buf.String(), Matches, `.*connected.*\n.*ended with: EOF\n`)
}

func (s *sessionSuite) TestSessionWireTimeout2(c *C) {
	nopTrack := NewTracker(nopLogger)
	errCh := make(chan error, 1)
	srv, cli, lst := serverClientWire()
	defer lst.Close()
	remSrv := &rememberDeadlineConn{srv, make([]string, 0, 2)}
	brkr := newTestBroker()
	go func() {
		errCh <- Session(remSrv, brkr, cfg5msPingInterval2msExchangeTout, nopTrack)
	}()
	io.WriteString(cli, "\x00")
	io.WriteString(cli, "\x00")
	err := <-errCh
	c.Check(err, FitsTypeOf, &net.OpError{})
	c.Check(remSrv.deadlineKind, DeepEquals, []string{"read", "both"})
	cli.Close()
}
