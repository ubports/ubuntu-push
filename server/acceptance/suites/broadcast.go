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
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/client/gethosts"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/api"
)

// BroadcastAcceptanceSuite has tests about broadcast.
type BroadcastAcceptanceSuite struct {
	AcceptanceSuite
}

func (s *BroadcastAcceptanceSuite) TestBroadcastToConnected(c *C) {
	events, errCh, stop := s.StartClient(c, "DEVB", nil)
	got, err := s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m1": 42}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))
	c.Check(NextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:1 payloads:[{"img1/m1":42}]`)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *BroadcastAcceptanceSuite) TestBroadcastToConnectedChannelFilter(c *C) {
	events, errCh, stop := s.StartClient(c, "DEVB", nil)
	got, err := s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m2": 10}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))
	got, err = s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m1": 20}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))
	c.Check(NextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:2 payloads:[{"img1/m1":20}]`)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *BroadcastAcceptanceSuite) TestBroadcastPending(c *C) {
	// send broadcast that will be pending
	got, err := s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m1": 1}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))

	events, errCh, stop := s.StartClient(c, "DEVB", nil)
	// gettting pending on connect
	c.Check(NextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:1 payloads:[{"img1/m1":1}]`)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *BroadcastAcceptanceSuite) TestBroadcastLargeNeedsSplitting(c *C) {
	// send bunch of broadcasts that will be pending
	payloadFmt := fmt.Sprintf(`{"img1/m1":%%d,"bloat":"%s"}`, strings.Repeat("x", 1024*2))
	for i := 0; i < 32; i++ {
		got, err := s.PostRequest("/broadcast", &api.Broadcast{
			Channel:  "system",
			ExpireOn: future,
			Data:     json.RawMessage(fmt.Sprintf(payloadFmt, i)),
		})
		c.Assert(err, IsNil, Commentf("%v", got))
	}

	events, errCh, stop := s.StartClient(c, "DEVC", nil)
	// gettting pending on connect
	n := 0
	for {
		evt := NextEvent(events, errCh)
		c.Check(evt, Matches, "broadcast chan:0 .*")
		n += 1
		if strings.Contains(evt, "topLevel:32") {
			break
		}
	}
	// was split
	c.Check(n > 1, Equals, true)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *BroadcastAcceptanceSuite) TestBroadcastDistribution2(c *C) {
	// start 1st client
	events1, errCh1, stop1 := s.StartClient(c, "DEV1", nil)
	// start 2nd client
	events2, errCh2, stop2 := s.StartClient(c, "DEV2", nil)
	// broadcast
	got, err := s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m1": 42}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))
	c.Check(NextEvent(events1, errCh1), Equals, `broadcast chan:0 app: topLevel:1 payloads:[{"img1/m1":42}]`)
	c.Check(NextEvent(events2, errCh2), Equals, `broadcast chan:0 app: topLevel:1 payloads:[{"img1/m1":42}]`)
	stop1()
	stop2()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh1), Equals, 0)
	c.Check(len(errCh2), Equals, 0)
}

func (s *BroadcastAcceptanceSuite) TestBroadcastFilterByLevel(c *C) {
	events, errCh, stop := s.StartClient(c, "DEVD", nil)
	got, err := s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m1": 1}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))
	c.Check(NextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:1 payloads:[{"img1/m1":1}]`)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
	// another broadcast
	got, err = s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m1": 2}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))
	// reconnect, provide levels, get only later notification
	events, errCh, stop = s.StartClient(c, "DEVD", map[string]int64{
		protocol.SystemChannelId: 1,
	})
	c.Check(NextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:2 payloads:[{"img1/m1":2}]`)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *BroadcastAcceptanceSuite) TestBroadcastTooAhead(c *C) {
	// send broadcasts that will be pending
	got, err := s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m1": 1}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))
	got, err = s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m1": 2}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))

	events, errCh, stop := s.StartClient(c, "DEVB", map[string]int64{
		protocol.SystemChannelId: 10,
	})
	// gettting last one pending on connect
	c.Check(NextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:2 payloads:[{"img1/m1":2}]`)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *BroadcastAcceptanceSuite) TestBroadcastTooAheadOnEmpty(c *C) {
	// nothing there
	events, errCh, stop := s.StartClient(c, "DEVB", map[string]int64{
		protocol.SystemChannelId: 10,
	})
	// gettting empty pending on connect
	c.Check(NextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:0 payloads:null`)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *BroadcastAcceptanceSuite) TestBroadcastWayBehind(c *C) {
	// send broadcasts that will be pending
	got, err := s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m1": 1}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))
	got, err = s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m1": 2}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))

	events, errCh, stop := s.StartClient(c, "DEVB", map[string]int64{
		protocol.SystemChannelId: -10,
	})
	// gettting pending on connect
	c.Check(NextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:2 payloads:[{"img1/m1":1},{"img1/m1":2}]`)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *BroadcastAcceptanceSuite) TestBroadcastExpiration(c *C) {
	// send broadcast that will be pending, and one that will expire
	got, err := s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"img1/m1": 1}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))
	got, err = s.PostRequest("/broadcast", &api.Broadcast{
		Channel:  "system",
		ExpireOn: time.Now().Add(1 * time.Second).Format(time.RFC3339),
		Data:     json.RawMessage(`{"img1/m1": 2}`),
	})
	c.Assert(err, IsNil, Commentf("%v", got))

	time.Sleep(2 * time.Second)
	// second broadcast is expired

	events, errCh, stop := s.StartClient(c, "DEVB", nil)
	// gettting pending on connect
	c.Check(NextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:2 payloads:[{"img1/m1":1}]`)
	stop()
	c.Assert(NextEvent(s.ServerEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

// test /delivery-hosts

func (s *BroadcastAcceptanceSuite) TestGetHosts(c *C) {
	gh := gethosts.New("", s.ServerAPIURL+"/delivery-hosts", 2*time.Second)
	host, err := gh.Get()
	c.Assert(err, IsNil)
	expected := &gethosts.Host{
		Domain: "push-delivery",
		Hosts:  []string{s.ServerAddr},
	}
	c.Check(host, DeepEquals, expected)
}
