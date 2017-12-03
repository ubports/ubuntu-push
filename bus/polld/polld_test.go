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

package polld

import (
	"testing"
	"time"

	. "launchpad.net/gocheck"

	testibus "github.com/ubports/ubuntu-push/bus/testing"
	helpers "github.com/ubports/ubuntu-push/testing"
	"github.com/ubports/ubuntu-push/testing/condition"
)

// hook up gocheck
func TestPolld(t *testing.T) { TestingT(t) }

type PdSuite struct {
	log *helpers.TestLogger
}

var _ = Suite(&PdSuite{})

func (s *PdSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
}

func (s *PdSuite) TestPollWorks(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(true))
	pd := New(endp, s.log)
	err := pd.Poll()
	c.Assert(err, IsNil)
	args := testibus.GetCallArgs(endp)
	c.Assert(args, HasLen, 1)
	c.Check(args[0].Member, Equals, "Poll")
	c.Check(args[0].Args, IsNil)
}

func (s *PdSuite) TestPollUnconfigured(c *C) {
	c.Check(new(polld).Poll(), Equals, ErrUnconfigured)
}

func (s *PdSuite) TestPollFails(c *C) {
	endp := testibus.NewTestingEndpoint(nil, condition.Work(false))
	pd := New(endp, s.log)
	c.Check(pd.Poll(), NotNil)
}

func (s *PdSuite) TestWatchDonesWorks(c *C) {
	endp := testibus.NewMultiValuedTestingEndpoint(nil, condition.Work(true), []interface{}{})
	pd := New(endp, s.log)
	ch, err := pd.WatchDones()
	c.Assert(err, IsNil)
	select {
	case b := <-ch:
		c.Check(b, Equals, true)
	case <-time.After(100 * time.Millisecond):
		c.Error("timeout waiting for bool")
	}
	select {
	case b := <-ch:
		c.Check(b, Equals, false)
	case <-time.After(100 * time.Millisecond):
		c.Error("timeout waiting for close")
	}
}

func (s *PdSuite) TestWatchDonesUnconfigured(c *C) {
	_, err := new(polld).WatchDones()
	c.Check(err, Equals, ErrUnconfigured)
}
