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

package util

import (
	"os"

	"gopkg.in/qml.v0"

	. "launchpad.net/gocheck"
)

type authSuite struct{}

var _ = Suite(&authSuite{})

func (s *authSuite) SetUpSuite(c *C) {
	if os.Getenv("PUSH_AUTH_TEST") == "1" {
		qml.Init(nil)
	}
}

func (s *authSuite) SetUpTest(c *C) {
	qml.SetLogger(c)
}

func (s *authSuite) TestGetAuth(c *C) {
	/*
	 * This test is only useful when the PUSH_AUTH_TEST environment
	 * variable is set to "1" - in which case the runner should have
	 * a Ubuntu One account setup via system-settings.
	 */
	if os.Getenv("PUSH_AUTH_TEST") != "1" {
		c.Skip("PUSH_AUTH_TEST not set to '1'")
	}
	auth, err := GetAuthorization()
	c.Assert(err, IsNil)
	c.Assert(auth, Matches, "OAuth .*oauth_consumer_key=.*")
}
