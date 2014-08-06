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
	"time"

	. "launchpad.net/gocheck"
)

type outSuite struct{}

var _ = Suite(&outSuite{})

func (*outSuite) TestCardGetTimestamp(c *C) {
	t := time.Now().Add(-2 * time.Second)
	c.Check(time.Unix((&Card{}).GetTimestamp(), 0).After(t), Equals, true)
	c.Check((&Card{Timestamp: 42}).GetTimestamp(), Equals, int64(42))
}
