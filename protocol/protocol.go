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

// Package protocol has code to talk the client-daemon<->push-server protocol.
package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

// Protocol is a connection capable of writing and reading the wire format of protocol messages.
type Protocol interface {
	SetDeadline(t time.Time)
	ReadMessage(msg interface{}) error
	WriteMessage(msg interface{}) error
}

func ReadWireFormatVersion(conn net.Conn, exchangeTimeout time.Duration) (ver int, err error) {
	var buf1 [1]byte
	err = conn.SetReadDeadline(time.Now().Add(exchangeTimeout))
	if err != nil {
		panic(fmt.Errorf("can't set deadline: %v", err))
	}
	_, err = conn.Read(buf1[:])
	ver = int(buf1[0])
	return
}

const ProtocolWireVersion = 0

// protocol0 handles version 0 of the wire format
type protocol0 struct {
	buffer *bytes.Buffer
	enc    *json.Encoder
	conn   net.Conn
}

// NewProtocol0 creates and initialises a protocol with wire format version 0.
func NewProtocol0(conn net.Conn) *protocol0 {
	buf := bytes.NewBuffer(make([]byte, 5000))
	return &protocol0{
		buffer: buf,
		enc:    json.NewEncoder(buf),
		conn:   conn}
}

// SetDeadline sets deadline for the subsquent WriteMessage/ReadMessage exchange
func (c *protocol0) SetDeadline(t time.Time) {
	err := c.conn.SetDeadline(t)
	if err != nil {
		panic(fmt.Errorf("can't set deadline: %v", err))
	}
}

// ReadMessage reads one message made of big endian uint16 length, JSON body of length from the connection.
func (c *protocol0) ReadMessage(msg interface{}) error {
	c.buffer.Reset()
	_, err := io.CopyN(c.buffer, c.conn, 2)
	if err != nil {
		return err
	}
	length := binary.BigEndian.Uint16(c.buffer.Bytes())
	c.buffer.Reset()
	_, err = io.CopyN(c.buffer, c.conn, int64(length))
	if err != nil {
		return err
	}
	return json.Unmarshal(c.buffer.Bytes(), msg)
}

// WriteMessage writes one message made of big endian uint16 length, JSON body of length to the connection.
func (c *protocol0) WriteMessage(msg interface{}) error {
	c.buffer.Reset()
	c.buffer.WriteString("\x00\x00") // placeholder for length
	err := c.enc.Encode(msg)
	if err != nil {
		panic(fmt.Errorf("WriteMessage got: %v", err))
	}
	msgLen := c.buffer.Len() - 3 // length space, extra newline
	toWrite := c.buffer.Bytes()
	binary.BigEndian.PutUint16(toWrite[:2], uint16(msgLen))
	_, err = c.conn.Write(toWrite[:msgLen+2])
	return err
}
