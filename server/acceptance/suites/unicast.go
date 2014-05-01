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
	//"fmt"
	//"strings"
	//"time"

	. "launchpad.net/gocheck"

	//"launchpad.net/ubuntu-push/protocol"
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
	return "", ""
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
