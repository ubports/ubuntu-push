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

// The client/session package handles the minutiae of interacting with
// the Ubuntu Push Notifications server.
package session

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"launchpad.net/ubuntu-push/client/session/levelmap"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	"net"
	"time"
)

var wireVersionBytes = []byte{protocol.ProtocolWireVersion}

type Notification struct {
	// something something something
}

type serverMsg struct {
	Type string `json:"T"`
	protocol.BroadcastMsg
	protocol.NotificationsMsg
}

// ClienSession holds a client<->server session and its configuration.
type ClientSession struct {
	// configuration
	DeviceId        string
	ServerAddr      string
	ExchangeTimeout time.Duration
	Levels          levelmap.LevelMap
	Protocolator    func(net.Conn) protocol.Protocol
	// connection
	Connection   net.Conn
	Log          logger.Logger
	TLS          *tls.Config
	proto        protocol.Protocol
	pingInterval time.Duration
	// status
	ErrCh chan error
	MsgCh chan *Notification
}

func NewSession(serverAddr string, pem []byte, exchangeTimeout time.Duration,
	deviceId string, log logger.Logger) (*ClientSession, error) {
	sess := &ClientSession{
		ExchangeTimeout: exchangeTimeout,
		ServerAddr:      serverAddr,
		DeviceId:        deviceId,
		Log:             log,
		Protocolator:    protocol.NewProtocol0,
		Levels:          levelmap.NewLevelMap(),
		TLS:             &tls.Config{InsecureSkipVerify: true}, // XXX
	}
	if pem != nil {
		cp := x509.NewCertPool()
		ok := cp.AppendCertsFromPEM(pem)
		if !ok {
			return nil, errors.New("could not parse certificate")
		}
		sess.TLS.RootCAs = cp
	}
	return sess, nil
}

// Dial connects to a server using the configuration in the ClientSession
// and sets up the connection.
func (sess *ClientSession) Dial() error {
	conn, err := net.DialTimeout("tcp", sess.ServerAddr, sess.ExchangeTimeout)
	if err != nil {
		return err
	}
	sess.Connection = tls.Client(conn, sess.TLS)
	return nil
}

func (sess *ClientSession) Close() {
	if sess.Connection != nil {
		sess.Connection.Close()
		// we ignore Close errors, on purpose (the thinking being that
		// the connection isn't really usable, and you've got nothing
		// you could do to recover at this stage).
		sess.Connection = nil
	}
}

// call this to ensure the session is sane to run
func (sess *ClientSession) checkRunnable() error {
	if sess.Connection == nil {
		return errors.New("can't run disconnected.")
	}
	if sess.Protocolator == nil {
		return errors.New("can't run without a protocol constructor.")
	}
	return nil
}

// handle "ping" messages
func (sess *ClientSession) handlePing() error {
	err := sess.Connection.SetDeadline(time.Now().Add(sess.ExchangeTimeout))
	if err == nil {
		err = sess.proto.WriteMessage(protocol.PingPongMsg{Type: "pong"})
		sess.Log.Debugf("Ping.")
	}
	return err
}

// handle "broadcast" messages
func (sess *ClientSession) handleBroadcast(bcast *serverMsg) error {
	err := sess.Connection.SetDeadline(time.Now().Add(sess.ExchangeTimeout))
	if err != nil {
		return err
	}
	err = sess.proto.WriteMessage(protocol.PingPongMsg{Type: "ack"})
	if err != nil {
		return err
	}
	sess.Log.Debugf("broadcast chan:%v app:%v topLevel:%d payloads:%s",
		bcast.ChanId, bcast.AppId, bcast.TopLevel, bcast.Payloads)
	if bcast.ChanId == protocol.SystemChannelId {
		// the system channel id, the only one we care about for now
		sess.Levels.Set(bcast.ChanId, bcast.TopLevel)
		sess.MsgCh <- &Notification{}
	} else {
		sess.Log.Debugf("What is this weird channel, %s?", bcast.ChanId)
	}
	return nil
}

// Run the session with the server, emits a stream of events.
func (sess *ClientSession) run() error {
	var err error
	var recv serverMsg
	conn := sess.Connection
	for {
		deadAfter := sess.pingInterval + sess.ExchangeTimeout
		conn.SetDeadline(time.Now().Add(deadAfter))
		err = sess.proto.ReadMessage(&recv)
		if err != nil {
			return err
		}
		switch recv.Type {
		case "ping":
			err = sess.handlePing()
		case "broadcast":
			err = sess.handleBroadcast(&recv)
		}
		if err != nil {
			return err
		}
	}
}
