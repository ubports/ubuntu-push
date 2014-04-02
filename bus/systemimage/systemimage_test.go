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

package systemimage

import (
	"testing"

	. "launchpad.net/gocheck"

	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

// hook up gocheck
func TestSystemImage(t *testing.T) { TestingT(t) }

type SISuite struct {
	log logger.Logger
}

var _ = Suite(&SISuite{})

func (s *SISuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
}

func (s *SISuite) TestWorks(c *C) {
	endp := testibus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{int32(101), "mako", "daily", "Unknown", map[string]string{}})
	si := New(endp, s.log)
	res, err := si.Info()
	c.Assert(err, IsNil)
	c.Check(res, DeepEquals, &InfoResult{
		BuildNumber:   101,
		Device:        "mako",
		Channel:       "daily",
		LastUpdate:    "Unknown",
		VersionDetail: map[string]string{},
	})
}

func (s *SISuite) TestFailsIfCallFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	si := New(endp, s.log)
	_, err := si.Info()
	c.Check(err, NotNil)
}
