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
	"time"

	. "launchpad.net/gocheck"

	clickhelp "launchpad.net/ubuntu-push/click/testing"
	helpers "launchpad.net/ubuntu-push/testing"
)

func takeNext(ch chan string, c *C) string {
	select {
	case s := <-ch:
		return s
	case <-time.After(5 * time.Second):
		c.Fatal("timed out waiting for value")
		return ""
	}
}

func Test(t *testing.T) { TestingT(t) }

type legacySuite struct {
	lhl *legacyHelperLauncher
	log *helpers.TestLogger
}

var _ = Suite(&legacySuite{})

func (ls *legacySuite) SetUpTest(c *C) {
	ls.log = helpers.NewTestLogger(c, "info")
	ls.lhl = New(ls.log)
}

func (ls *legacySuite) TestInstallObserver(c *C) {
	c.Check(ls.lhl.done, IsNil)
	c.Check(ls.lhl.InstallObserver(func(string) {}), IsNil)
	c.Check(ls.lhl.done, NotNil)
}

func (s *legacySuite) TestHelperInfo(c *C) {
	appname := "ubuntu-system-settings"
	app := clickhelp.MustParseAppId("_" + appname)
	hid, hex := s.lhl.HelperInfo(app)
	c.Check(hid, Equals, "")
	c.Check(hex, Equals, filepath.Join(legacyHelperDir, appname))
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

	id2 := takeNext(ch, c)
	c.Check(id, Equals, id2)

	d2, err := ioutil.ReadFile(f2)
	c.Assert(err, IsNil)
	c.Check(string(d2), Equals, string(d1))

}

func (ls *legacySuite) TestLaunchFails(c *C) {
	_, err := ls.lhl.Launch("", "/does/not/exist", "", "")
	c.Assert(err, NotNil)
}

func (ls *legacySuite) TestHelperFails(c *C) {
	ch := make(chan string, 1)
	c.Assert(ls.lhl.InstallObserver(func(id string) { ch <- id }), IsNil)

	_, err := ls.lhl.Launch("", "/bin/false", "", "")
	c.Assert(err, IsNil)

	takeNext(ch, c)
	c.Check(ls.log.Captured(), Matches, "(?s).*Legacy helper failed.*")
}

func (ls *legacySuite) TestHelperFailsLog(c *C) {
	ch := make(chan string, 1)
	c.Assert(ls.lhl.InstallObserver(func(id string) { ch <- id }), IsNil)

	exe := helpers.ScriptAbsPath("noisy-helper.sh")
	_, err := ls.lhl.Launch("", exe, "", "")
	c.Assert(err, IsNil)

	takeNext(ch, c)
	c.Check(ls.log.Captured(), Matches, "(?s).*BOOM-1.*")
	c.Check(ls.log.Captured(), Matches, "(?s).*BANG-1.*")
	c.Check(ls.log.Captured(), Matches, "(?s).*BOOM-20.*")
	c.Check(ls.log.Captured(), Matches, "(?s).*BANG-20.*")
}

func (ls *legacySuite) TestStop(c *C) {
	ch := make(chan string, 1)
	c.Assert(ls.lhl.InstallObserver(func(id string) { ch <- id }), IsNil)

	// 	exe := helpers.ScriptAbsPath("slow-helper.sh")
	id, err := ls.lhl.Launch("", "/bin/sleep", "9", "1")
	c.Assert(err, IsNil)

	err = ls.lhl.Stop("", "===")
	c.Check(err, NotNil) // not a valid id

	err = ls.lhl.Stop("", id)
	c.Check(err, IsNil)
	takeNext(ch, c)
	err = ls.lhl.Stop("", id)
	c.Check(err, NotNil) // no such processs
}
