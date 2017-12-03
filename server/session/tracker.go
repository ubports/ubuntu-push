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
	"fmt"
	"net"
	"time"

	"github.com/ubports/ubuntu-push/logger"
	"github.com/ubports/ubuntu-push/server/broker"
)

// SessionTracker logs session events.
type SessionTracker interface {
	logger.Logger
	// Session got started.
	Start(WithRemoteAddr)
	// SessionId
	SessionId() string
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
	sessionId string
}

func NewTracker(logger logger.Logger) SessionTracker {
	return &tracker{Logger: logger}
}

func (trk *tracker) Start(conn WithRemoteAddr) {
	trk.sessionId = fmt.Sprintf("%x", time.Now().UnixNano()-sessionsEpoch)
	trk.Debugf("session(%s) connected %v", trk.sessionId, conn.RemoteAddr())
}

func (trk *tracker) SessionId() string {
	return trk.sessionId
}

func (trk *tracker) Registered(sess broker.BrokerSession) {
	trk.Infof("session(%s) registered %v", trk.sessionId, sess.DeviceIdentifier())
}

func (trk *tracker) EffectivePingInterval(time.Duration) {
}

func (trk *tracker) End(err error) error {
	trk.Debugf("session(%s) ended with: %v", trk.sessionId, err)
	return err
}
