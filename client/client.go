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

package client

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	"net"
	"time"
)

var wireVersionBytes = []byte{protocol.ProtocolWireVersion}

type Notification struct {
	// something something something
}

type LevelMap interface {
	Set(level string, top int64)
	GetAll() map[string]int64
}

type mapLevelMap map[string]int64

func (m *mapLevelMap) Set(level string, top int64) {
	(*m)[level] = top
}
func (m *mapLevelMap) GetAll() map[string]int64 {
	return map[string]int64(*m)
}

var _ LevelMap = &mapLevelMap{}

type Config struct {
	// session configuration
	ExchangeTimeout config.ConfigTimeDuration `json:"exchange_timeout"`
	// server connection config
	Addr        config.ConfigHostPort
	CertPEMFile string `json:"cert_pem_file"`
}

// ClienSession holds a client<->server session and its configuration.
type ClientSession struct {
	// configuration
	DeviceId        string
	ServerAddr      string
	ExchangeTimeout time.Duration
	Levels          LevelMap
	// connection
	Connection   net.Conn
	Protocolator func(net.Conn) protocol.Protocol
	Log          logger.Logger
	TLS          *tls.Config
	// status
	ErrCh chan error
	MsgCh chan *Notification
}

func NewSession(config Config, log logger.Logger, deviceId string) (*ClientSession, error) {
	sess := &ClientSession{
		ExchangeTimeout: config.ExchangeTimeout.TimeDuration(),
		ServerAddr:      config.Addr.HostPort(),
		DeviceId:        deviceId,
		Log:             log,
		Protocolator:    protocol.NewProtocol0,
		Levels:          &mapLevelMap{},
		TLS:             &tls.Config{InsecureSkipVerify: true}, // XXX
	}
	if config.CertPEMFile != "" {
		cert, err := ioutil.ReadFile(config.CertPEMFile)
		if err != nil {
			return nil, err
		}
		cp := x509.NewCertPool()
		ok := cp.AppendCertsFromPEM(cert)
		if !ok {
			return nil, errors.New("dial: could not parse certificate")
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

type serverMsg struct {
	Type string `json:"T"`
	protocol.BroadcastMsg
	protocol.NotificationsMsg
}

func (sess *ClientSession) Reset() error {
	if sess.Protocolator == nil {
		return errors.New("Can't Reset() without a protocol constructor.")
	}
	if sess.Connection != nil {
		sess.Connection.Close() // just in case
	}
	err := sess.Dial()
	if err != nil {
		sess.Log.Errorf("%s", err)
		return err
	}
	sess.ErrCh = make(chan error, 1)
	sess.MsgCh = make(chan *Notification)
	sess.Run()
	return nil
}

func (sess *ClientSession) Run() {
	go func() { sess.ErrCh <- sess.run() }()
}

// Run the session with the server, emits a stream of events.
func (sess *ClientSession) run() error {
	conn := sess.Connection
	if conn == nil {
		return errors.New("Can't run() disconnected.")
	}
	if sess.Protocolator == nil {
		return errors.New("Can't run() without a protocol constructor.")
	}
	defer conn.Close()
	err := conn.SetDeadline(time.Now().Add(sess.ExchangeTimeout))
	if err != nil {
		return err
	}
	_, err = conn.Write(wireVersionBytes)
	// The Writer docs: Write must return a non-nil error if it returns
	// n < len(p). So, no need to check number of bytes written, hooray.
	if err != nil {
		return err
	}
	proto := sess.Protocolator(conn)
	err = proto.WriteMessage(protocol.ConnectMsg{
		Type:     "connect",
		DeviceId: sess.DeviceId,
		Levels:   sess.Levels.GetAll(),
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
	sess.Log.Debugf("Connected %v.", conn.LocalAddr())
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
			sess.Log.Debugf("Ping.")
		case "broadcast":
			conn.SetDeadline(time.Now().Add(sess.ExchangeTimeout))
			err := proto.WriteMessage(protocol.PingPongMsg{Type: "ack"})
			if err != nil {
				return err
			}
			sess.Log.Debugf("broadcast chan:%v app:%v topLevel:%d payloads:%s",
				recv.ChanId, recv.AppId, recv.TopLevel, recv.Payloads)
			if recv.ChanId == protocol.SystemChannelId {
				// the system channel id, the only one we care about for now
				sess.Levels.Set(recv.ChanId, recv.TopLevel)
				sess.MsgCh <- &Notification{}
			} else {
				sess.Log.Debugf("What is this weird channel, %s?", recv.ChanId)
			}
		}
	}
}
