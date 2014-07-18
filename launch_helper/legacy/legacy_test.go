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

package legacy

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	. "launchpad.net/gocheck"

	helpers "launchpad.net/ubuntu-push/testing"
)

func Test(t *testing.T) { TestingT(t) }

type legacySuite struct {
	lhl *legacyHelperLauncher
}

var _ = Suite(&legacySuite{})

func (ls *legacySuite) SetUpTest(c *C) {
	ls.lhl = New()
}

func (ls *legacySuite) TestInstallObserver(c *C) {
	c.Check(ls.lhl.done, IsNil)
	c.Check(ls.lhl.InstallObserver(func(string) {}), IsNil)
	c.Check(ls.lhl.done, NotNil)
}

func (ls *legacySuite) TestLaunch(c *C) {
	ch := make(chan string, 1)
	c.Assert(ls.lhl.InstallObserver(func(id string) { ch <- id }), IsNil)

	d := c.MkDir()
	f1 := filepath.Join(d, "one")
	f2 := filepath.Join(d, "two")

	d1 := []byte(`potato`)
	c.Assert(ioutil.WriteFile(f1, d1, 0644), IsNil)

	exe := helpers.ScriptAbsPath("trivial-helper.sh")
	id, err := ls.lhl.Launch("", exe, f1, f2)
	c.Assert(err, IsNil)
	c.Check(id, Not(Equals), "")

	id2 := <-ch
	c.Check(id, Equals, id2)

	d2, err := ioutil.ReadFile(f2)
	c.Assert(err, IsNil)
	c.Check(string(d2), Equals, string(d1))

}
