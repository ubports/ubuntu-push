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

// Package session has code handling long-lived connections from devices.
package session

import (
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/broker"
	"net"
	"time"
)

// SessionConfig is for carrying the session configuration.
type SessionConfig interface {
	// pings are emitted each ping interval
	PingInterval() time.Duration
	// send and waiting for response shouldn't take more than exchange
	// timeout
	ExchangeTimeout() time.Duration
}

// sessionStart manages the start of the protocol session.
func sessionStart(proto protocol.Protocol, brkr broker.Broker, cfg SessionConfig) (broker.BrokerSession, error) {
	var connMsg protocol.ConnectMsg
	proto.SetDeadline(time.Now().Add(cfg.ExchangeTimeout()))
	err := proto.ReadMessage(&connMsg)
	if err != nil {
		return nil, err
	}
	if connMsg.Type != "connect" {
		return nil, &broker.ErrAbort{"expected CONNECT message"}
	}
	return brkr.Register(&connMsg)
}

// exchange writes outMsg message, reads answer in inMsg
func exchange(proto protocol.Protocol, outMsg, inMsg interface{}, exchangeTimeout time.Duration) error {
	proto.SetDeadline(time.Now().Add(exchangeTimeout))
	err := proto.WriteMessage(outMsg)
	if err != nil {
		return err
	}
	err = proto.ReadMessage(inMsg)
	if err != nil {
		return err
	}
	return nil
}

// sessionLoop manages the exchanges of the protocol session.
func sessionLoop(proto protocol.Protocol, sess broker.BrokerSession, cfg SessionConfig) error {
	pingInterval := cfg.PingInterval()
	exchangeTimeout := cfg.ExchangeTimeout()
	pingTimer := time.NewTimer(pingInterval)
	ch := sess.SessionChannel()
	for {
		select {
		case <-pingTimer.C:
			pingMsg := &protocol.PingPongMsg{"ping"}
			var pongMsg protocol.PingPongMsg
			err := exchange(proto, pingMsg, &pongMsg, exchangeTimeout)
			if err != nil {
				return err
			}
			if pongMsg.Type != "pong" {
				return &broker.ErrAbort{"expected PONG message"}
			}
			pingTimer.Reset(pingInterval)
		case exchg := <-ch:
			// xxx later can use ch closing for shutdown/reset
			pingTimer.Stop()
			outMsg, inMsg, err := exchg.Prepare(sess)
			if err != nil {
				return err
			}
			for {
				done := outMsg.Split()
				err = exchange(proto, outMsg, inMsg, exchangeTimeout)
				if err != nil {
					return err
				}
				if done {
					pingTimer.Reset(pingInterval)
				}
				err = exchg.Acked(sess)
				if err != nil {
					return err
				}
				if done {
					break
				}
			}
		}
	}
	return nil
}

var sessionsEpoch = time.Date(2013, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

// sessionTracker logs session events.
type sessionTracker struct {
	logger.Logger
	sessionId int64 // xxx use timeuuid later
}

func (trk *sessionTracker) start(conn interface {
	RemoteAddr() net.Addr
}) {
	trk.sessionId = time.Now().UnixNano() - sessionsEpoch
	trk.Debugf("session(%x) connected %v", trk.sessionId, conn.RemoteAddr())
}

func (trk *sessionTracker) registered(sess broker.BrokerSession) {
	trk.Infof("session(%x) registered %v", trk.sessionId, sess.DeviceId())
}

func (trk *sessionTracker) end(err error) error {
	trk.Debugf("session(%x) ended with: %v", trk.sessionId, err)
	return err
}

// Session manages the session with a client.
func Session(conn net.Conn, brkr broker.Broker, cfg SessionConfig, logger logger.Logger) error {
	defer conn.Close()
	track := sessionTracker{Logger: logger}
	track.start(conn)
	v, err := protocol.ReadWireFormatVersion(conn, cfg.ExchangeTimeout())
	if err != nil {
		return track.end(err)
	}
	if v != protocol.ProtocolWireVersion {
		return track.end(&broker.ErrAbort{"unexpected wire format version"})
	}
	proto := protocol.NewProtocol0(conn)
	sess, err := sessionStart(proto, brkr, cfg)
	if err != nil {
		return track.end(err)
	}
	track.registered(sess)
	defer brkr.Unregister(sess)
	return track.end(sessionLoop(proto, sess, cfg))
}
