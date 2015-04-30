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

package broker

import (
	"encoding/json"
	"fmt"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/protocol"
)

type brokerSuite struct{}

var _ = Suite(&brokerSuite{})

func (s *brokerSuite) TestErrAbort(c *C) {
	err := &ErrAbort{"expected FOO"}
	c.Check(fmt.Sprintf("%s", err), Equals, "session aborted (expected FOO)")
}

func (s *brokerSuite) TestGetInfoString(c *C) {
	connectMsg := &protocol.ConnectMsg{}
	v, err := GetInfoString(connectMsg, "foo", "?")
	c.Check(err, IsNil)
	c.Check(v, Equals, "?")

	connectMsg.Info = map[string]interface{}{"foo": "yay"}
	v, err = GetInfoString(connectMsg, "foo", "?")
	c.Check(err, IsNil)
	c.Check(v, Equals, "yay")

	connectMsg.Info["foo"] = 33
	v, err = GetInfoString(connectMsg, "foo", "?")
	c.Check(err, Equals, ErrUnexpectedValue)
}

func (s *brokerSuite) TestGetInfoInt(c *C) {
	connectMsg := &protocol.ConnectMsg{}
	v, err := GetInfoInt(connectMsg, "bar", -1)
	c.Check(err, IsNil)
	c.Check(v, Equals, -1)

	err = json.Unmarshal([]byte(`{"bar": 233}`), &connectMsg.Info)
	c.Assert(err, IsNil)
	v, err = GetInfoInt(connectMsg, "bar", -1)
	c.Check(err, IsNil)
	c.Check(v, Equals, 233)

	connectMsg.Info["bar"] = "garbage"
	v, err = GetInfoInt(connectMsg, "bar", -1)
	c.Check(err, Equals, ErrUnexpectedValue)
}
