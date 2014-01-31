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
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/server/broker"
	"net"
	"time"
)

// SessionTracker logs session events.
type SessionTracker interface {
	logger.Logger
	// Session got started.
	Start(WithRemoteAddr)
	// Session got registered with broker as sess BrokerSession.
	Registered(sess broker.BrokerSession)
	// Report effective elapsed ping interval.
	EffectivePingInterval(time.Duration)
	// Session got ended with error err (can be nil).
	End(error) error
}

// WithRemoteAddr can report a remote address.
type WithRemoteAddr interface {
	RemoteAddr() net.Addr
}

var sessionsEpoch = time.Date(2013, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

// Tracker implements SessionTracker simply.
type tracker struct {
	logger.Logger
	sessionId int64 // xxx use timeuuid later
}

func NewTracker(logger logger.Logger) SessionTracker {
	return &tracker{Logger: logger}
}

func (trk *tracker) Start(conn WithRemoteAddr) {
	trk.sessionId = time.Now().UnixNano() - sessionsEpoch
	trk.Debugf("session(%x) connected %v", trk.sessionId, conn.RemoteAddr())
}

func (trk *tracker) Registered(sess broker.BrokerSession) {
	trk.Infof("session(%x) registered %v", trk.sessionId, sess.DeviceIdentifier())
}

func (trk *tracker) EffectivePingInterval(time.Duration) {
}

func (trk *tracker) End(err error) error {
	trk.Debugf("session(%x) ended with: %v", trk.sessionId, err)
	return err
}
