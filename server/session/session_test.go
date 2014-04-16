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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	stdtesting "testing"
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/broker"
	"launchpad.net/ubuntu-push/server/broker/testing"
	helpers "launchpad.net/ubuntu-push/testing"
)

func TestSession(t *stdtesting.T) { TestingT(t) }

type sessionSuite struct {
	testlog *helpers.TestLogger
}

func (s *sessionSuite) SetUpTest(c *C) {
	s.testlog = helpers.NewTestLogger(c, "debug")
}

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

var cfg10msPingInterval5msExchangeTout = &testSessionConfig{
	pingInterval:    10 * time.Millisecond,
	exchangeTimeout: 5 * time.Millisecond,
}

type testBroker struct {
	registration chan interface{}
	err          error
}

func newTestBroker() *testBroker {
	return &testBroker{registration: make(chan interface{}, 2)}
}

func (tb *testBroker) Register(connect *protocol.ConnectMsg) (broker.BrokerSession, error) {
	tb.registration <- "register " + connect.DeviceId
	return &testing.TestBrokerSession{DeviceId: connect.DeviceId}, tb.err
}

func (tb *testBroker) Unregister(sess broker.BrokerSession) {
	tb.registration <- "unregister " + sess.DeviceIdentifier()
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
		sess, err = sessionStart(tp, brkr, cfg10msPingInterval5msExchangeTout)
		errCh <- err
	}()
	c.Check(takeNext(down), Equals, "deadline 5ms")
	up <- protocol.ConnectMsg{Type: "connect", ClientVer: "1", DeviceId: "dev-1"}
	c.Check(takeNext(down), Equals, protocol.ConnAckMsg{
		Type:   "connack",
		Params: protocol.ConnAckParams{(10 * time.Millisecond).String()},
	})
	up <- nil // no write error
	err := <-errCh
	c.Check(err, IsNil)
	c.Check(takeNext(brkr.registration), Equals, "register dev-1")
	c.Check(sess.DeviceIdentifier(), Equals, "dev-1")
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
		sess, err = sessionStart(tp, brkr, cfg10msPingInterval5msExchangeTout)
		errCh <- err
	}()
	up <- protocol.ConnectMsg{Type: "connect", ClientVer: "1", DeviceId: "dev-1"}
	takeNext(down) // CONNACK
	up <- nil      // no write error
	err := <-errCh
	c.Check(err, Equals, errRegister)
}

func (s *sessionSuite) TestSessionStartReadError(c *C) {
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	up <- io.ErrUnexpectedEOF
	_, err := sessionStart(tp, nil, cfg10msPingInterval5msExchangeTout)
	c.Check(err, Equals, io.ErrUnexpectedEOF)
}

func (s *sessionSuite) TestSessionStartWriteError(c *C) {
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	up <- protocol.ConnectMsg{Type: "connect"}
	up <- io.ErrUnexpectedEOF
	_, err := sessionStart(tp, nil, cfg10msPingInterval5msExchangeTout)
	c.Check(err, Equals, io.ErrUnexpectedEOF)
	// sanity
	c.Check(takeNext(down), Matches, "deadline.*")
	c.Check(takeNext(down), FitsTypeOf, protocol.ConnAckMsg{})
}

func (s *sessionSuite) TestSessionStartMismatch(c *C) {
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	up <- protocol.ConnectMsg{Type: "what"}
	_, err := sessionStart(tp, nil, cfg10msPingInterval5msExchangeTout)
	c.Check(err, DeepEquals, &broker.ErrAbort{"expected CONNECT message"})
}

var cfg5msPingInterval2msExchangeTout = &testSessionConfig{
	pingInterval:    5 * time.Millisecond,
	exchangeTimeout: 2 * time.Millisecond,
}

func (s *sessionSuite) TestSessionLoop(c *C) {
	track := &testTracker{NewTracker(s.testlog), make(chan interface{}, 2)}
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	sess := &testing.TestBrokerSession{}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, track)
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
	c.Check(track.interval, HasLen, 2)
	c.Check((<-track.interval).(time.Duration) <= 8*time.Millisecond, Equals, true)
	c.Check((<-track.interval).(time.Duration) <= 8*time.Millisecond, Equals, true)
}

func (s *sessionSuite) TestSessionLoopWriteError(c *C) {
	nopTrack := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	sess := &testing.TestBrokerSession{}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, nopTrack)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), FitsTypeOf, protocol.PingPongMsg{})
	up <- io.ErrUnexpectedEOF // write error
	err := <-errCh
	c.Check(err, Equals, io.ErrUnexpectedEOF)
}

func (s *sessionSuite) TestSessionLoopMismatch(c *C) {
	nopTrack := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	sess := &testing.TestBrokerSession{}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, nopTrack)
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
	done     chan interface{}
}

func (exchg *testExchange) Prepare(sess broker.BrokerSession) (outMsg protocol.SplittableMsg, inMsg interface{}, err error) {
	return &testMsg{Type: "msg", nParts: exchg.nParts}, &exchg.inMsg, exchg.prepErr
}

func (exchg *testExchange) Acked(sess broker.BrokerSession, done bool) error {
	time.Sleep(exchg.finSleep)
	if exchg.done != nil {
		var doneStr string
		if done {
			doneStr = "y"
		} else {
			doneStr = "n"
		}
		exchg.done <- doneStr
	}
	return exchg.finErr
}

func (s *sessionSuite) TestSessionLoopExchange(c *C) {
	nopTrack := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	exchanges <- &testExchange{}
	sess := &testing.TestBrokerSession{Exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, nopTrack)
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

func (s *sessionSuite) TestSessionLoopKick(c *C) {
	nopTrack := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	sess := &testing.TestBrokerSession{Exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, nopTrack)
	}()
	close(exchanges)
	err := <-errCh
	c.Check(err, DeepEquals, &broker.ErrAbort{"terminated"})
}

func (s *sessionSuite) TestSessionLoopExchangeErrNop(c *C) {
	nopTrack := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	exchanges <- &testExchange{prepErr: broker.ErrNop}
	sess := &testing.TestBrokerSession{Exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, nopTrack)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, protocol.PingPongMsg{Type: "ping"})
	up <- nil // no write error
	up <- io.EOF
	err := <-errCh
	c.Check(err, Equals, io.EOF)
}

func (s *sessionSuite) TestSessionLoopExchangeSplit(c *C) {
	nopTrack := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	exchange := &testExchange{nParts: 2, done: make(chan interface{}, 2)}
	exchanges <- exchange
	sess := &testing.TestBrokerSession{Exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, nopTrack)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, testMsg{Type: "msg", Part: 1, nParts: 2})
	up <- nil // no write error
	up <- testMsg{Type: "ack"}
	c.Check(takeNext(exchange.done), Equals, "n")
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, testMsg{Type: "msg", Part: 2, nParts: 2})
	up <- nil // no write error
	up <- testMsg{Type: "ack"}
	c.Check(takeNext(exchange.done), Equals, "y")
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, protocol.PingPongMsg{Type: "ping"})
	up <- nil // no write error
	up <- io.EOF
	err := <-errCh
	c.Check(err, Equals, io.EOF)
}

func (s *sessionSuite) TestSessionLoopExchangePrepareError(c *C) {
	nopTrack := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	prepErr := errors.New("prepare failure")
	exchanges <- &testExchange{prepErr: prepErr}
	sess := &testing.TestBrokerSession{Exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, nopTrack)
	}()
	err := <-errCh
	c.Check(err, Equals, prepErr)
}

func (s *sessionSuite) TestSessionLoopExchangeAckedError(c *C) {
	nopTrack := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	finErr := errors.New("finish error")
	exchanges <- &testExchange{finErr: finErr}
	sess := &testing.TestBrokerSession{Exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, nopTrack)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, testMsg{Type: "msg"})
	up <- nil // no write error
	up <- testMsg{Type: "ack"}
	err := <-errCh
	c.Check(err, Equals, finErr)
}

func (s *sessionSuite) TestSessionLoopExchangeWriteError(c *C) {
	nopTrack := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	exchanges <- &testExchange{}
	sess := &testing.TestBrokerSession{Exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, nopTrack)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), FitsTypeOf, testMsg{})
	up <- io.ErrUnexpectedEOF
	err := <-errCh
	c.Check(err, Equals, io.ErrUnexpectedEOF)
}

func (s *sessionSuite) TestSessionLoopConnBrokenExchange(c *C) {
	nopTrack := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	msg := &protocol.ConnBrokenMsg{"connbroken", "BREASON"}
	exchanges <- &broker.ConnMetaExchange{msg}
	sess := &testing.TestBrokerSession{Exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, nopTrack)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, protocol.ConnBrokenMsg{"connbroken", "BREASON"})
	up <- nil // no write error
	err := <-errCh
	c.Check(err, DeepEquals, &broker.ErrAbort{"session broken for reason"})
}

func (s *sessionSuite) TestSessionLoopConnWarnExchange(c *C) {
	nopTrack := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	msg := &protocol.ConnWarnMsg{"connwarn", "WREASON"}
	exchanges <- &broker.ConnMetaExchange{msg}
	sess := &testing.TestBrokerSession{Exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg5msPingInterval2msExchangeTout, nopTrack)
	}()
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, protocol.ConnWarnMsg{"connwarn", "WREASON"})
	up <- nil // no write error
	// session continues
	c.Check(takeNext(down), Equals, "deadline 2ms")
	c.Check(takeNext(down), DeepEquals, protocol.PingPongMsg{Type: "ping"})
	up <- nil // no write error
	up <- io.EOF
	err := <-errCh
	c.Check(err, Equals, io.EOF)
}

type testTracker struct {
	SessionTracker
	interval chan interface{}
}

func (trk *testTracker) EffectivePingInterval(interval time.Duration) {
	trk.interval <- interval
}

var cfg50msPingInterval = &testSessionConfig{
	pingInterval:    50 * time.Millisecond,
	exchangeTimeout: 10 * time.Millisecond,
}

func (s *sessionSuite) TestSessionLoopExchangeNextPing(c *C) {
	track := &testTracker{NewTracker(s.testlog), make(chan interface{}, 1)}
	errCh := make(chan error, 1)
	up := make(chan interface{}, 5)
	down := make(chan interface{}, 5)
	tp := &testProtocol{up, down}
	exchanges := make(chan broker.Exchange, 1)
	exchanges <- &testExchange{finSleep: 15 * time.Millisecond}
	sess := &testing.TestBrokerSession{Exchanges: exchanges}
	go func() {
		errCh <- sessionLoop(tp, sess, cfg50msPingInterval, track)
	}()
	c.Check(takeNext(down), Equals, "deadline 10ms")
	c.Check(takeNext(down), DeepEquals, testMsg{Type: "msg"})
	up <- nil // no write error
	up <- testMsg{Type: "ack"}
	// next ping interval starts around here
	interval := takeNext(track.interval).(time.Duration)
	c.Check(takeNext(down), Equals, "deadline 10ms")
	c.Check(takeNext(down), DeepEquals, protocol.PingPongMsg{Type: "ping"})
	effectiveOfPing := float64(interval) / float64(50*time.Millisecond)
	comment := Commentf("effectiveOfPing=%f", effectiveOfPing)
	c.Check(effectiveOfPing > 0.95, Equals, true, comment)
	c.Check(effectiveOfPing < 1.15, Equals, true, comment)
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
func (s *sessionSuite) TestSessionWire(c *C) {
	track := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	srv, cli, lst := serverClientWire()
	defer lst.Close()
	remSrv := &rememberDeadlineConn{srv, make([]string, 0, 2)}
	brkr := newTestBroker()
	go func() {
		errCh <- Session(remSrv, brkr, cfg50msPingInterval, track)
	}()
	io.WriteString(cli, "\x00")
	io.WriteString(cli, "\x00\x20{\"T\":\"connect\",\"DeviceId\":\"DEV\"}")
	// connack
	downStream := bufio.NewReader(cli)
	msg, err := downStream.ReadBytes(byte('}'))
	c.Check(err, IsNil)
	c.Check(msg, DeepEquals, []byte("\x00\x30{\"T\":\"connack\",\"Params\":{\"PingInterval\":\"50ms\"}"))
	// eat the last }
	rbr, err := downStream.ReadByte()
	c.Check(err, IsNil)
	c.Check(rbr, Equals, byte('}'))
	// first ping
	msg, err = downStream.ReadBytes(byte('}'))
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
	c.Check(s.testlog.Captured(), Matches, `.*connected.*\n.*registered DEV.*\n.*ended with: EOF\n`)
}

func (s *sessionSuite) TestSessionWireTimeout(c *C) {
	nopTrack := NewTracker(s.testlog)
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
	track := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	srv, cli, lst := serverClientWire()
	defer lst.Close()
	brkr := newTestBroker()
	go func() {
		errCh <- Session(srv, brkr, cfg50msPingInterval, track)
	}()
	io.WriteString(cli, "\x10")
	err := <-errCh
	c.Check(err, DeepEquals, &broker.ErrAbort{"unexpected wire format version"})
	cli.Close()
	// tracking
	c.Check(s.testlog.Captured(), Matches, `.*connected.*\n.*ended with: session aborted \(unexpected.*version\)\n`)

}

func (s *sessionSuite) TestSessionWireEarlyClose(c *C) {
	track := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	srv, cli, lst := serverClientWire()
	defer lst.Close()
	brkr := newTestBroker()
	go func() {
		errCh <- Session(srv, brkr, cfg50msPingInterval, track)
	}()
	cli.Close()
	err := <-errCh
	c.Check(err, Equals, io.EOF)
	// tracking
	c.Check(s.testlog.Captured(), Matches, `.*connected.*\n.*ended with: EOF\n`)

}

func (s *sessionSuite) TestSessionWireEarlyClose2(c *C) {
	track := NewTracker(s.testlog)
	errCh := make(chan error, 1)
	srv, cli, lst := serverClientWire()
	defer lst.Close()
	brkr := newTestBroker()
	go func() {
		errCh <- Session(srv, brkr, cfg50msPingInterval, track)
	}()
	io.WriteString(cli, "\x00")
	io.WriteString(cli, "\x00")
	cli.Close()
	err := <-errCh
	c.Check(err, Equals, io.EOF)
	// tracking
	c.Check(s.testlog.Captured(), Matches, `.*connected.*\n.*ended with: EOF\n`)
}

func (s *sessionSuite) TestSessionWireTimeout2(c *C) {
	nopTrack := NewTracker(s.testlog)
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
