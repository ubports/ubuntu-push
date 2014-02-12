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

package protocol

import (
	"encoding/binary"
	"encoding/json"
	// "fmt"
	"io"
	"net"
	"testing"
	"time"

	. "launchpad.net/gocheck"
)

func TestProtocol(t *testing.T) { TestingT(t) }

type protocolSuite struct{}

var _ = Suite(&protocolSuite{})

type deadline struct {
	kind      string
	deadAfter time.Duration
}

func (d *deadline) setDeadAfter(t time.Time) {
	deadAfter := t.Sub(time.Now())
	d.deadAfter = (deadAfter + time.Millisecond/2) / time.Millisecond * time.Millisecond
}

type rw struct {
	buf []byte
	n   int
	err error
}

type testConn struct {
	deadlines []*deadline
	reads     []rw
	writes    []*rw
}

func (tc *testConn) LocalAddr() net.Addr {
	return nil
}

func (tc *testConn) RemoteAddr() net.Addr {
	return nil
}

func (tc *testConn) Close() error {
	return nil
}

func (tc *testConn) SetDeadline(t time.Time) error {
	deadline := tc.deadlines[0]
	deadline.kind = "both"
	deadline.setDeadAfter(t)
	tc.deadlines = tc.deadlines[1:]
	return nil
}

func (tc *testConn) SetReadDeadline(t time.Time) error {
	deadline := tc.deadlines[0]
	deadline.kind = "read"
	deadline.setDeadAfter(t)
	tc.deadlines = tc.deadlines[1:]
	return nil
}

func (tc *testConn) SetWriteDeadline(t time.Time) error {
	deadline := tc.deadlines[0]
	deadline.kind = "write"
	deadline.setDeadAfter(t)
	tc.deadlines = tc.deadlines[1:]
	return nil
}

func (tc *testConn) Read(buf []byte) (n int, err error) {
	read := tc.reads[0]
	copy(buf, read.buf)
	tc.reads = tc.reads[1:]
	return read.n, read.err
}

func (tc *testConn) Write(buf []byte) (n int, err error) {
	write := tc.writes[0]
	n = copy(write.buf, buf)
	write.buf = write.buf[:n]
	write.n = n
	err = write.err
	tc.writes = tc.writes[1:]
	return
}

func (s *protocolSuite) TestReadWireFormatVersion(c *C) {
	deadl := deadline{}
	read1 := rw{buf: []byte{42}, n: 1}
	tc := &testConn{reads: []rw{read1}, deadlines: []*deadline{&deadl}}
	ver, err := ReadWireFormatVersion(tc, time.Minute)
	c.Check(err, IsNil)
	c.Check(ver, Equals, 42)
	c.Check(deadl.kind, Equals, "read")
	c.Check(deadl.deadAfter, Equals, time.Minute)
}

func (s *protocolSuite) TestReadWireFormatVersionError(c *C) {
	deadl := deadline{}
	read1 := rw{err: io.EOF}
	tc := &testConn{reads: []rw{read1}, deadlines: []*deadline{&deadl}}
	_, err := ReadWireFormatVersion(tc, time.Minute)
	c.Check(err, Equals, io.EOF)
}

func (s *protocolSuite) TestSetDeadline(c *C) {
	deadl := deadline{}
	tc := &testConn{deadlines: []*deadline{&deadl}}
	pc := NewProtocol0(tc)
	pc.SetDeadline(time.Now().Add(time.Minute))
	c.Check(deadl.kind, Equals, "both")
	c.Check(deadl.deadAfter, Equals, time.Minute)
}

type testMsg struct {
	Type string `json:"T"`
	A    uint64
}

func lengthAsBytes(length uint16) []byte {
	var buf [2]byte
	var res = buf[:]
	binary.BigEndian.PutUint16(res, length)
	return res
}

func (s *protocolSuite) TestReadMessage(c *C) {
	msgBuf, _ := json.Marshal(testMsg{Type: "msg", A: 2000})
	readMsgLen := rw{buf: lengthAsBytes(uint16(len(msgBuf))), n: 2}
	readMsgBody := rw{buf: msgBuf, n: len(msgBuf)}
	tc := &testConn{reads: []rw{readMsgLen, readMsgBody}}
	pc := NewProtocol0(tc)
	var recvMsg testMsg
	err := pc.ReadMessage(&recvMsg)
	c.Check(err, IsNil)
	c.Check(recvMsg, DeepEquals, testMsg{Type: "msg", A: 2000})
}

func (s *protocolSuite) TestReadMessageBits(c *C) {
	msgBuf, _ := json.Marshal(testMsg{Type: "msg", A: 2000})
	readMsgLen := rw{buf: lengthAsBytes(uint16(len(msgBuf))), n: 2}
	readMsgBody1 := rw{buf: msgBuf[:5], n: 5}
	readMsgBody2 := rw{buf: msgBuf[5:], n: len(msgBuf) - 5}
	tc := &testConn{reads: []rw{readMsgLen, readMsgBody1, readMsgBody2}}
	pc := NewProtocol0(tc)
	var recvMsg testMsg
	err := pc.ReadMessage(&recvMsg)
	c.Check(err, IsNil)
	c.Check(recvMsg, DeepEquals, testMsg{Type: "msg", A: 2000})
}

func (s *protocolSuite) TestReadMessageIOErrors(c *C) {
	msgBuf, _ := json.Marshal(testMsg{Type: "msg", A: 2000})
	readMsgLenErr := rw{n: 1, err: io.ErrClosedPipe}
	tc1 := &testConn{reads: []rw{readMsgLenErr}}
	pc1 := NewProtocol0(tc1)
	var recvMsg testMsg
	err := pc1.ReadMessage(&recvMsg)
	c.Check(err, Equals, io.ErrClosedPipe)

	readMsgLen := rw{buf: lengthAsBytes(uint16(len(msgBuf))), n: 2}
	readMsgBody1 := rw{buf: msgBuf[:5], n: 5}
	readMsgBody2Err := rw{n: 2, err: io.EOF}
	tc2 := &testConn{reads: []rw{readMsgLen, readMsgBody1, readMsgBody2Err}}
	pc2 := NewProtocol0(tc2)
	err = pc2.ReadMessage(&recvMsg)
	c.Check(err, Equals, io.EOF)
}

func (s *protocolSuite) TestReadMessageBrokenJSON(c *C) {
	msgBuf := []byte("{\"T\"}")
	readMsgLen := rw{buf: lengthAsBytes(uint16(len(msgBuf))), n: 2}
	readMsgBody := rw{buf: msgBuf, n: len(msgBuf)}
	tc := &testConn{reads: []rw{readMsgLen, readMsgBody}}
	pc := NewProtocol0(tc)
	var recvMsg testMsg
	err := pc.ReadMessage(&recvMsg)
	c.Check(err, FitsTypeOf, &json.SyntaxError{})
}

func (s *protocolSuite) TestWriteMessage(c *C) {
	writeMsg := rw{buf: make([]byte, 64)}
	tc := &testConn{writes: []*rw{&writeMsg}}
	pc := NewProtocol0(tc)
	msg := testMsg{Type: "m", A: 9999}
	err := pc.WriteMessage(&msg)
	c.Check(err, IsNil)
	var msgLen int = int(binary.BigEndian.Uint16(writeMsg.buf[:2]))
	c.Check(msgLen, Equals, len(writeMsg.buf)-2)
	var wroteMsg testMsg
	formatErr := json.Unmarshal(writeMsg.buf[2:], &wroteMsg)
	c.Check(formatErr, IsNil)
	c.Check(wroteMsg, DeepEquals, testMsg{Type: "m", A: 9999})
}

func (s *protocolSuite) TestWriteMessageIOErrors(c *C) {
	writeMsgErr := rw{buf: make([]byte, 0), err: io.ErrClosedPipe}
	tc1 := &testConn{writes: []*rw{&writeMsgErr}}
	pc1 := NewProtocol0(tc1)
	msg := testMsg{Type: "m", A: 9999}
	err := pc1.WriteMessage(&msg)
	c.Check(err, Equals, io.ErrClosedPipe)
}
