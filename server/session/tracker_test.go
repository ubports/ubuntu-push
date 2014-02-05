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
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/server/broker"
	"launchpad.net/ubuntu-push/server/broker/testing"
	helpers "launchpad.net/ubuntu-push/testing"
	"net"
)

type trackerSuite struct {
	testlog *helpers.TestLogger
}

func (s *trackerSuite) SetUpTest(c *C) {
	s.testlog = helpers.NewTestLogger(c, "debug")
}

var _ = Suite(&trackerSuite{})

type testRemoteAddrable struct{}

func (tra *testRemoteAddrable) RemoteAddr() net.Addr {
	return &net.TCPAddr{net.IPv4(127, 0, 0, 1), 9999, ""}
}

func (s *trackerSuite) TestSessionTrackStart(c *C) {
	track := NewTracker(s.testlog)
	track.Start(&testRemoteAddrable{})
	c.Check(track.(*tracker).sessionId, Not(Equals), 0)
	regExpected := fmt.Sprintf(`DEBUG session\(%x\) connected 127\.0\.0\.1:9999\n`, track.(*tracker).sessionId)
	c.Check(s.testlog.Captured(), Matches, regExpected)
}

func (s *trackerSuite) TestSessionTrackRegistered(c *C) {
	track := NewTracker(s.testlog)
	track.Start(&testRemoteAddrable{})
	track.Registered(&testing.TestBrokerSession{DeviceId: "DEV-ID"})
	regExpected := fmt.Sprintf(`.*connected.*\nINFO session\(%x\) registered DEV-ID\n`, track.(*tracker).sessionId)
	c.Check(s.testlog.Captured(), Matches, regExpected)
}

func (s *trackerSuite) TestSessionTrackEnd(c *C) {
	track := NewTracker(s.testlog)
	track.Start(&testRemoteAddrable{})
	track.End(&broker.ErrAbort{})
	regExpected := fmt.Sprintf(`.*connected.*\nDEBUG session\(%x\) ended with: session aborted \(\)\n`, track.(*tracker).sessionId)
	c.Check(s.testlog.Captured(), Matches, regExpected)
}
