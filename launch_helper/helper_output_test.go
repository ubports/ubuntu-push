/*
 Copyright 2014 Canonical Ltd.

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

package launch_helper

import (
	"encoding/json"
	"time"

	. "launchpad.net/gocheck"
)

type outSuite struct{}

var _ = Suite(&outSuite{})

func (*outSuite) TestCardGetTimestamp(c *C) {
	t := time.Now().Add(-2 * time.Second)
	var card Card
	err := json.Unmarshal([]byte(`{"timestamp": 12}`), &card)
	c.Assert(err, IsNil)
	c.Check(card, DeepEquals, Card{RawTimestamp: 12})
	c.Check(time.Unix((&Card{}).Timestamp(), 0).After(t), Equals, true)
	c.Check((&Card{RawTimestamp: 42}).Timestamp(), Equals, int64(42))
}

func (*outSuite) TestBadVibeBegetsNilVibe(c *C) {
	for _, s := range []string{
		`{}`,
		`{"vibrate": "foo"}`,
		`{"vibrate": {}}`,
		`{"vibrate": false}`,         // not bad, but rather pointless
		`{"vibrate": {"repeat": 2}}`, // no pattern
		`{"vibrate": {"repeat": "foo"}}`,
		`{"vibrate": {"pattern": "foo"}}`,
		`{"vibrate": {"pattern": ["foo"]}}`,
		`{"vibrate": {"pattern": null}}`,
		`{"vibrate": {"pattern": [-1]}}`,
		`{"vibrate": {"pattern": [1], "repeat": -1}}`,
	} {
		var notif *Notification
		err := json.Unmarshal([]byte(s), &notif)
		c.Assert(err, IsNil)
		c.Assert(notif, NotNil)
		c.Check(notif.Vibration(nil), IsNil, Commentf("not nil Vibration() for: %s", s))
		c.Check(notif.Vibration(nil), IsNil, Commentf("not nil second call to Vibration() for: %s", s))
	}
}

func (*outSuite) TestGoodVibe(c *C) {
	var notif *Notification
	err := json.Unmarshal([]byte(`{"vibrate": {"pattern": [1,2,3], "repeat": 2}}`), &notif)
	c.Assert(err, IsNil)
	c.Assert(notif, NotNil)
	c.Check(notif.Vibration(nil), DeepEquals, &Vibration{Pattern: []uint32{1, 2, 3}, Repeat: 2})
}

func (*outSuite) TestGoodSimpleVibe(c *C) {
	var notif *Notification
	fallback := &Vibration{Pattern: []uint32{100, 100}, Repeat: 3}
	err := json.Unmarshal([]byte(`{"vibrate": true}`), &notif)
	c.Assert(err, IsNil)
	c.Assert(notif, NotNil)
	c.Check(notif.Vibration(fallback), Equals, fallback)
}
