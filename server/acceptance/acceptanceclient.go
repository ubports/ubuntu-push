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

package acceptance

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"launchpad.net/ubuntu-push/protocol"
	"net"
	"time"
)

var wireVersionBytes = []byte{protocol.ProtocolWireVersion}

// ClienSession holds a client<->server session and its configuration.
type ClientSession struct {
	// configuration
	DeviceId        string
	ServerAddr      string
	PingInterval    time.Duration
	ExchangeTimeout time.Duration
	CertPEMBlock    []byte
	ReportPings     bool
	Levels          map[string]int64
	Insecure        bool // don't verify certs
	// connection
	Connection net.Conn
}

// Dial connects to a server using the configuration in the ClientSession
// and sets up the connection.
func (sess *ClientSession) Dial() error {
	conn, err := net.DialTimeout("tcp", sess.ServerAddr, sess.ExchangeTimeout)
	if err != nil {
		return err
	}
	tlsConfig := &tls.Config{}
	if sess.CertPEMBlock != nil {
		cp := x509.NewCertPool()
		ok := cp.AppendCertsFromPEM(sess.CertPEMBlock)
		if !ok {
			return errors.New("dial: could not parse certificate")
		}
		tlsConfig.RootCAs = cp
	}
	tlsConfig.InsecureSkipVerify = sess.Insecure
	sess.Connection = tls.Client(conn, tlsConfig)
	return nil
}

type serverMsg struct {
	Type string `json:"T"`
	protocol.BroadcastMsg
	protocol.NotificationsMsg
}

// Run the session with the server, emits a stream of events.
func (sess *ClientSession) Run(events chan<- string) error {
	conn := sess.Connection
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(sess.ExchangeTimeout))
	_, err := conn.Write(wireVersionBytes)
	if err != nil {
		return err
	}
	proto := protocol.NewProtocol0(conn)
	err = proto.WriteMessage(protocol.ConnectMsg{
		Type:     "connect",
		DeviceId: sess.DeviceId,
		Levels:   sess.Levels,
	})
	if err != nil {
		return err
	}
	events <- fmt.Sprintf("connected %v", conn.LocalAddr())
	var recv serverMsg
	for {
		deadAfter := sess.PingInterval + sess.ExchangeTimeout
		conn.SetDeadline(time.Now().Add(deadAfter))
		err = proto.ReadMessage(&recv)
		if err != nil {
			return err
		}
		switch recv.Type {
		case "ping":
			conn.SetDeadline(time.Now().Add(sess.ExchangeTimeout))
			err := proto.WriteMessage(protocol.PingPongMsg{Type: "pong"})
			if err != nil {
				return err
			}
			if sess.ReportPings {
				events <- "Ping"
			}
		case "broadcast":
			conn.SetDeadline(time.Now().Add(sess.ExchangeTimeout))
			err := proto.WriteMessage(protocol.PingPongMsg{Type: "ack"})
			if err != nil {
				return err
			}
			pack, err := json.Marshal(recv.Payloads)
			if err != nil {
				return err
			}
			events <- fmt.Sprintf("broadcast chan:%v app:%v topLevel:%d payloads:%s", recv.ChanId, recv.AppId, recv.TopLevel, pack)
		}
	}
	return nil
}
