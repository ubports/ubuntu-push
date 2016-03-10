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
	m := map[string]string{
		"version_detail":        "ubuntu=20160304.2,device=20160304.2,custom=20160304.2,version=381",
		"last_update_date":      "2016-03-04 15:25:31",
		"last_check_date":       "2016-03-08 04:30:34",
		"target_version_detail": "-1",
		"device_name":           "mako",
		"target_build_number":   "-1",
		"channel_name":          "ubuntu-touch/rc-proposed/ubuntu",
		"current_build_number":  "381",
	}
	endp := testibus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{m})
	si := New(endp, s.log)
	res, err := si.Information()
	c.Assert(err, IsNil)
	c.Check(res, DeepEquals, &InfoResult{
		BuildNumber: 381,
		Device:      "mako",
		Channel:     "ubuntu-touch/rc-proposed/ubuntu",
		LastUpdate:  "2016-03-04 15:25:31",
		VersionDetail: map[string]string{
			"ubuntu":  "20160304.2",
			"device":  "20160304.2",
			"custom":  "20160304.2",
			"version": "381",
		},
		Raw: m,
	})
}

func (s *SISuite) TestFailsIfCallFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	si := New(endp, s.log)
	_, err := si.Information()
	c.Check(err, NotNil)
}
