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

package server

import (
	"net"
	"testing"

	. "launchpad.net/gocheck"

	helpers "github.com/ubports/ubuntu-push/testing"
)

func TestRunners(t *testing.T) { TestingT(t) }

type bootlogSuite struct{}

var _ = Suite(&bootlogSuite{})

func (s *bootlogSuite) TestBootLogListener(c *C) {
	prevBootLogger := BootLogger
	testlog := helpers.NewTestLogger(c, "info")
	BootLogger = testlog
	defer func() {
		BootLogger = prevBootLogger
	}()
	lst, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, IsNil)
	defer lst.Close()
	BootLogListener("client", lst)
	c.Check(testlog.Captured(), Matches, "INFO listening for client on "+lst.Addr().String()+"\n")
}
