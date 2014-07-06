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

package sounds

import (
	"testing"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/launch_helper"
	helpers "launchpad.net/ubuntu-push/testing"
)

func TestSounds(t *testing.T) { TestingT(t) }

type soundsSuite struct {
	log *helpers.TestLogger
}

var _ = Suite(&soundsSuite{})

func (ss *soundsSuite) SetUpTest(c *C) {
	ss.log = helpers.NewTestLogger(c, "debug")
}

func (ss *soundsSuite) TestNew(c *C) {
	s := New(ss.log)
	c.Check(s.log, Equals, ss.log)
	c.Check(s.player, Equals, "paplay")
}

func (ss *soundsSuite) TestPresent(c *C) {
	s := &Sound{player: "echo", log: ss.log}

	c.Check(s.Present("", "", &launch_helper.Notification{Sound: "hello"}), Equals, true)
	c.Check(ss.log.Captured(), Matches, `(?sm).* playing sound hello using echo`)
}

func (ss *soundsSuite) TestPresentFails(c *C) {
	s := &Sound{player: "/", log: ss.log}

	// nil notification
	c.Check(s.Present("", "", nil), Equals, false)
	// no Sound
	c.Check(s.Present("", "", &launch_helper.Notification{}), Equals, false)
	// bad player
	c.Check(s.Present("", "", &launch_helper.Notification{Sound: "hello"}), Equals, false)
}
