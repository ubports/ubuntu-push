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

package suites

import (
	"encoding/json"
	"fmt"
	"strings"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/server/api"
)

// UnicastAcceptanceSuite has tests about unicast.
type UnicastAcceptanceSuite struct {
	AcceptanceSuite
	AssociatedAuth func(string) (string, string)
}

func (s *UnicastAcceptanceSuite) associatedAuth(deviceId string) (userId string, auth string) {
	if s.AssociatedAuth != nil {
		return s.AssociatedAuth(deviceId)
	}
	return deviceId, ""
}

func (s *UnicastAcceptanceSuite) TestUnicastToConnected(c *C) {
	userId, auth := s.associatedAuth("DEV1")
	events, errCh, stop := s.StartClientAuth(c, "DEV1", nil, auth)
	got, err := s.PostRequest("/notify", &api.Unicast{
		UserId:   userId,
		DeviceId: "DEV1",
		AppId:    "app1",
		ExpireOn: future,
		Data:     json.RawMessage(`{"a": 42}`),
	})
	c.Assert(err, IsNil)
	c.Assert(got, Matches, ".*ok.*")
	c.Check(NextEvent(events, errCh), Equals, `unicast app:app1 payload:{"a":42};`)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *UnicastAcceptanceSuite) TestUnicastCorrectDistribution(c *C) {
	userId1, auth1 := s.associatedAuth("DEV1")
	userId2, auth2 := s.associatedAuth("DEV2")
	// start 1st client
	events1, errCh1, stop1 := s.StartClientAuth(c, "DEV1", nil, auth1)
	// start 2nd client
	events2, errCh2, stop2 := s.StartClientAuth(c, "DEV2", nil, auth2)
	// unicast to one and the other
	got, err := s.PostRequest("/notify", &api.Unicast{
		UserId:   userId1,
		DeviceId: "DEV1",
		AppId:    "app1",
		ExpireOn: future,
		Data:     json.RawMessage(`{"to": 1}`),
	})
	c.Assert(err, IsNil)
	c.Assert(got, Matches, ".*ok.*")
	got, err = s.PostRequest("/notify", &api.Unicast{
		UserId:   userId2,
		DeviceId: "DEV2",
		AppId:    "app1",
		ExpireOn: future,
		Data:     json.RawMessage(`{"to": 2}`),
	})
	c.Assert(err, IsNil)
	c.Assert(got, Matches, ".*ok.*")
	c.Check(NextEvent(events1, errCh1), Equals, `unicast app:app1 payload:{"to":1};`)
	c.Check(NextEvent(events2, errCh2), Equals, `unicast app:app1 payload:{"to":2};`)
	stop1()
	stop2()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh1), Equals, 0)
	c.Check(len(errCh2), Equals, 0)
}

func (s *UnicastAcceptanceSuite) TestUnicastPending(c *C) {
	// send unicast that will be pending
	userId, auth := s.associatedAuth("DEV1")
	got, err := s.PostRequest("/notify", &api.Unicast{
		UserId:   userId,
		DeviceId: "DEV1",
		AppId:    "app1",
		ExpireOn: future,
		Data:     json.RawMessage(`{"a": 42}`),
	})
	c.Assert(err, IsNil)
	c.Assert(got, Matches, ".*ok.*")

	// get pending on connect
	events, errCh, stop := s.StartClientAuth(c, "DEV1", nil, auth)
	c.Check(NextEvent(events, errCh), Equals, `unicast app:app1 payload:{"a":42};`)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *UnicastAcceptanceSuite) TestUnicastLargeNeedsSplitting(c *C) {
	userId, auth := s.associatedAuth("DEV2")
	// send bunch of unicasts that will be pending
	payloadFmt := fmt.Sprintf(`{"serial":%%d,"bloat":"%s"}`, strings.Repeat("x", 1024*2))
	for i := 0; i < 32; i++ {
		got, err := s.PostRequest("/notify", &api.Unicast{
			UserId:   userId,
			DeviceId: "DEV2",
			AppId:    "app1",
			ExpireOn: future,
			Data:     json.RawMessage(fmt.Sprintf(payloadFmt, i)),
		})
		c.Assert(err, IsNil)
		c.Assert(got, Matches, ".*ok.*")
	}

	events, errCh, stop := s.StartClientAuth(c, "DEV2", nil, auth)
	// gettting pending on connect
	n := 0
	for {
		evt := NextEvent(events, errCh)
		c.Check(evt, Matches, "unicast app:app1 .*")
		n += 1
		if strings.Contains(evt, `"serial":31`) {
			break
		}
	}
	// was split
	c.Check(n > 1, Equals, true)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}
