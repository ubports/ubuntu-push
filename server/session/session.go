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
	"errors"
	"net"
	"time"

	"github.com/ubports/ubuntu-push/protocol"
	"github.com/ubports/ubuntu-push/server/broker"
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
func sessionStart(proto protocol.Protocol, brkr broker.Broker, cfg SessionConfig, track SessionTracker) (broker.BrokerSession, error) {
	var connMsg protocol.ConnectMsg
	proto.SetDeadline(time.Now().Add(cfg.ExchangeTimeout()))
	err := proto.ReadMessage(&connMsg)
	if err != nil {
		return nil, err
	}
	if connMsg.Type != "connect" {
		return nil, &broker.ErrAbort{"expected CONNECT message"}
	}
	err = proto.WriteMessage(&protocol.ConnAckMsg{
		Type:   "connack",
		Params: protocol.ConnAckParams{PingInterval: cfg.PingInterval().String()},
	})
	if err != nil {
		return nil, err
	}
	return brkr.Register(&connMsg, track)
}

var errOneway = errors.New("oneway")

type loop struct {
	// params
	proto protocol.Protocol
	sess  broker.BrokerSession
	track SessionTracker
	// exchange timeout
	exchangeTimeout time.Duration
	// ping mgmt
	pingInterval  time.Duration
	pingTimer     *time.Timer
	intervalStart time.Time
}

// exchange writes outMsg message, reads answer in inMsg
func (l *loop) exchange(outMsg, inMsg interface{}) error {
	proto := l.proto
	proto.SetDeadline(time.Now().Add(l.exchangeTimeout))
	err := proto.WriteMessage(outMsg)
	if err != nil {
		return err
	}
	if inMsg == nil { // no answer expected
		if outMsg.(protocol.OnewayMsg).OnewayContinue() {
			return errOneway
		}
		return &broker.ErrAbort{"session broken for reason"}
	}
	err = proto.ReadMessage(inMsg)
	if err != nil {
		return err
	}
	return nil
}

func (l *loop) pingTimerReset(totalReset bool) bool {
	interval := l.pingInterval
	if !totalReset {
		interval -= time.Since(l.intervalStart)
		if interval <= 0 {
			return false // late
		}
	}
	l.pingTimer.Reset(interval)
	if totalReset {
		l.intervalStart = time.Now()
	}
	return true
}

func (l *loop) doPing() error {
	l.track.EffectivePingInterval(time.Since(l.intervalStart))
	pingMsg := &protocol.PingPongMsg{"ping"}
	var pongMsg protocol.PingPongMsg
	err := l.exchange(pingMsg, &pongMsg)
	if err != nil {
		return err
	}
	if pongMsg.Type != "pong" {
		return &broker.ErrAbort{"expected PONG message"}
	}
	l.pingTimerReset(true)
	return nil
}

func (l *loop) run() error {
	ch := l.sess.SessionChannel()
Loop:
	for {
		select {
		case <-l.pingTimer.C:
			err := l.doPing()
			if err != nil {
				return err
			}
		case exchg := <-ch:
			l.pingTimer.Stop()
			if exchg == nil {
				return &broker.ErrAbort{"terminated"}
			}
			outMsg, inMsg, err := exchg.Prepare(l.sess)
			if err == broker.ErrNop { // nothing to do
				if !l.pingTimerReset(false) {
					// we are late, do a ping here
					err := l.doPing()
					if err != nil {
						return err
					}
				}
				continue Loop
			}
			if err != nil {
				return err
			}
			for {
				done := outMsg.Split()
				err = l.exchange(outMsg, inMsg)
				if err == errOneway {
					l.pingTimerReset(true)
					continue Loop
				}
				if err != nil {
					return err
				}
				if done {
					l.pingTimerReset(true)
				}
				err = exchg.Acked(l.sess, done)
				if err != nil {
					return err
				}
				if done {
					break
				}
			}
		}
	}
}

// sessionLoop manages the exchanges of the protocol session.
func sessionLoop(proto protocol.Protocol, sess broker.BrokerSession, cfg SessionConfig, track SessionTracker) error {
	pingInterval := cfg.PingInterval()
	l := &loop{
		proto: proto,
		sess:  sess,
		track: track,
		// ping setup
		pingInterval:    pingInterval,
		pingTimer:       time.NewTimer(pingInterval),
		intervalStart:   time.Now(),
		exchangeTimeout: cfg.ExchangeTimeout(),
	}
	return l.run()
}

// Session manages the session with a client.
func Session(conn net.Conn, brkr broker.Broker, cfg SessionConfig, track SessionTracker) error {
	defer conn.Close()
	track.Start(conn)
	v, err := protocol.ReadWireFormatVersion(conn, cfg.ExchangeTimeout())
	if err != nil {
		return track.End(err)
	}
	if v != protocol.ProtocolWireVersion {
		return track.End(&broker.ErrAbort{"unexpected wire format version"})
	}
	proto := protocol.NewProtocol0(conn)
	sess, err := sessionStart(proto, brkr, cfg, track)
	if err != nil {
		return track.End(err)
	}
	track.Registered(sess)
	defer brkr.Unregister(sess)
	return track.End(sessionLoop(proto, sess, cfg, track))
}
