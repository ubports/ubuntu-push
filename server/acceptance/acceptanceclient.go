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

// Package acceptance contains the acceptance client.
package acceptance

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"launchpad.net/ubuntu-push/protocol"
)

var wireVersionBytes = []byte{protocol.ProtocolWireVersion}

// ClienSession holds a client<->server session and its configuration.
type ClientSession struct {
	// configuration
	DeviceId        string
	Model           string
	ImageChannel    string
	ServerAddr      string
	ExchangeTimeout time.Duration
	CertPEMBlock    []byte
	ReportPings     bool
	Levels          map[string]int64
	Insecure        bool   // don't verify certs
	Prefix          string // prefix for events
	Auth            string
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
	protocol.ConnWarnMsg
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
		Info: map[string]interface{}{
			"device":  sess.Model,
			"channel": sess.ImageChannel,
		},
		Authorization: sess.Auth,
	})
	if err != nil {
		return err
	}
	var connAck protocol.ConnAckMsg
	err = proto.ReadMessage(&connAck)
	if err != nil {
		return err
	}
	pingInterval, err := time.ParseDuration(connAck.Params.PingInterval)
	if err != nil {
		return err
	}
	events <- fmt.Sprintf("%sconnected %v", sess.Prefix, conn.LocalAddr())
	var recv serverMsg
	for {
		deadAfter := pingInterval + sess.ExchangeTimeout
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
				events <- sess.Prefix + "ping"
			}
		case "notifications":
			conn.SetDeadline(time.Now().Add(sess.ExchangeTimeout))
			err := proto.WriteMessage(protocol.AckMsg{Type: "ack"})
			if err != nil {
				return err
			}
			parts := make([]string, len(recv.Notifications))
			for i, notif := range recv.Notifications {
				pack, err := json.Marshal(&notif.Payload)
				if err != nil {
					return err
				}
				parts[i] = fmt.Sprintf("app:%v payload:%s;", notif.AppId, pack)
			}
			events <- fmt.Sprintf("%sunicast %s", sess.Prefix, strings.Join(parts, " "))
		case "broadcast":
			conn.SetDeadline(time.Now().Add(sess.ExchangeTimeout))
			err := proto.WriteMessage(protocol.AckMsg{Type: "ack"})
			if err != nil {
				return err
			}
			pack, err := json.Marshal(recv.Payloads)
			if err != nil {
				return err
			}
			events <- fmt.Sprintf("%sbroadcast chan:%v app:%v topLevel:%d payloads:%s", sess.Prefix, recv.ChanId, recv.AppId, recv.TopLevel, pack)
		case "connwarn":
			events <- fmt.Sprintf("%sconnwarn %s", sess.Prefix, recv.Reason)
		}
	}
	return nil
}
