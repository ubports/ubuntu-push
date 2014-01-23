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
	"io/ioutil"
	. "launchpad.net/gocheck"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/testing/condition"
	"testing"
	"time"
)

// hook up gocheck
func TestRedialer(t *testing.T) { TestingT(t) }

type RedialerSuite struct {
	timeouts []time.Duration
}

var nullog = logger.NewSimpleLogger(ioutil.Discard, "error")
var _ = Suite(&RedialerSuite{})

func (s *RedialerSuite) SetUpSuite(c *C) {
	skipTimeout = true
}

func (s *RedialerSuite) TearDownSuite(c *C) {
	skipTimeout = false
}

func (s *RedialerSuite) TestWorks(c *C) {
	endp := testibus.NewTestingEndpoint(condition.Fail2Work(3), nil)
	// instead of bus.Dial(), we do AutoRedial(bus)
	c.Check(AutoRedial(endp), Equals, uint32(4))
}
