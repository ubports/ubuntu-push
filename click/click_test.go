/*
 Copyright 2013-2015 Canonical Ltd.

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

package click

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/click/cappinfo"
)

func TestClick(t *testing.T) { TestingT(t) }

type clickSuite struct{}

var _ = Suite(&clickSuite{})

func GetPyVer() string {
	out, _ := exec.Command("python3", "-V").Output()
	pyver := strings.Replace(string(out[:]), "Python ", "", -1)
	vers := strings.Split(pyver, ".")
	return fmt.Sprintf("%s.%s", vers[0], vers[1])
}

func (cs *clickSuite) TestParseAppId(c *C) {
	app, err := ParseAppId("com.ubuntu.clock_clock")
	c.Assert(err, IsNil)
	c.Check(app.Package, Equals, "com.ubuntu.clock")
	c.Check(app.InPackage("com.ubuntu.clock"), Equals, true)
	c.Check(app.Application, Equals, "clock")
	c.Check(app.Version, Equals, "")
	c.Check(app.Click, Equals, true)
	c.Check(app.Original(), Equals, "com.ubuntu.clock_clock")
	c.Check(fmt.Sprintf("%s", app), Equals, "com.ubuntu.clock_clock")
	c.Check(app.DispatchPackage(), Equals, "com.ubuntu.clock")

	app, err = ParseAppId("com.ubuntu.clock_clock_10")
	c.Assert(err, IsNil)
	c.Check(app.Package, Equals, "com.ubuntu.clock")
	c.Check(app.InPackage("com.ubuntu.clock"), Equals, true)
	c.Check(app.Application, Equals, "clock")
	c.Check(app.Version, Equals, "10")
	c.Check(app.Click, Equals, true)
	c.Check(app.Original(), Equals, "com.ubuntu.clock_clock_10")
	c.Check(fmt.Sprintf("%s", app), Equals, "com.ubuntu.clock_clock_10")
	c.Check(app.Versioned(), Equals, "com.ubuntu.clock_clock_10")
	c.Check(app.Base(), Equals, "com.ubuntu.clock_clock")
	c.Check(app.DesktopId(), Equals, "com.ubuntu.clock_clock_10.desktop")
	c.Check(app.DispatchPackage(), Equals, "com.ubuntu.clock")

	for _, s := range []string{"com.ubuntu.clock_clock_10_4", "com.ubuntu.clock", ""} {
		app, err = ParseAppId(s)
		c.Check(app, IsNil)
		c.Check(err, Equals, ErrInvalidAppId)
	}
}

func (cs *clickSuite) TestVersionedPanic(c *C) {
	app, err := ParseAppId("com.ubuntu.clock_clock")
	c.Assert(err, IsNil)
	c.Check(func() { app.Versioned() }, PanicMatches, `Versioned\(\) on AppId without version/not verified:.*`)
}

func (cs *clickSuite) TestParseAppIdLegacy(c *C) {
	pyver := fmt.Sprintf("python%s", GetPyVer())
	app, err := ParseAppId(fmt.Sprintf("_%s", pyver))
	c.Assert(err, IsNil)
	c.Check(app.Package, Equals, "")
	c.Check(app.InPackage(""), Equals, true)
	c.Check(app.Application, Equals, pyver)
	c.Check(app.Version, Equals, "")
	c.Check(app.Click, Equals, false)
	c.Check(app.Original(), Equals, fmt.Sprintf("_%s", pyver))
	c.Check(app.Versioned(), Equals, pyver)
	c.Check(app.Base(), Equals, pyver)
	c.Check(app.DesktopId(), Equals, fmt.Sprintf("%s.desktop", pyver))
	c.Check(app.DispatchPackage(), Equals, pyver)

	for _, s := range []string{"_.foo", "_foo/", "_/foo"} {
		app, err = ParseAppId(s)
		c.Check(app, IsNil)
		c.Check(err, Equals, ErrInvalidAppId)
	}
}

func (cs *clickSuite) TestJSON(c *C) {
	pyver := fmt.Sprintf("python%s", GetPyVer())
	for _, appId := range []string{"com.ubuntu.clock_clock", "com.ubuntu.clock_clock_10", fmt.Sprintf("_%s", pyver)} {
		app, err := ParseAppId(appId)
		c.Assert(err, IsNil, Commentf(appId))
		b, err := json.Marshal(app)
		c.Assert(err, IsNil, Commentf(appId))
		var vapp *AppId
		err = json.Unmarshal(b, &vapp)
		c.Assert(err, IsNil, Commentf(appId))
		c.Check(vapp, DeepEquals, app)
	}
}

func (cs *clickSuite) TestIcon(c *C) {
	pyver := fmt.Sprintf("python%s", GetPyVer())
	app, err := ParseAppId(fmt.Sprintf("_%s", pyver))
	c.Assert(err, IsNil)
	c.Check(app.Icon(), Equals, fmt.Sprintf("/usr/share/pixmaps/%s.xpm", pyver))
}

func (s *clickSuite) TestUser(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	c.Assert(u, NotNil)
}

func (s *clickSuite) TestInstalledNegative(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	app, err := ParseAppId("com.foo.bar_baz")
	c.Assert(err, IsNil)
	c.Check(u.Installed(app, false), Equals, false)
}

func (s *clickSuite) TestInstalledVersionNegative(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	app, err := ParseAppId("com.ubuntu.clock_clock_1000.0")
	c.Assert(err, IsNil)
	c.Check(u.Installed(app, false), Equals, false)
}

func (s *clickSuite) TestInstalledClock(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	ver := u.ccu.CGetVersion("com.ubuntu.clock")
	if ver == "" {
		c.Skip("no com.ubuntu.clock pkg installed")
	}
	app, err := ParseAppId("com.ubuntu.clock_clock")
	c.Assert(err, IsNil)
	c.Check(u.Installed(app, false), Equals, true)
	app, err = ParseAppId("com.ubuntu.clock_clock_" + ver)
	c.Assert(err, IsNil)
	c.Check(u.Installed(app, false), Equals, true)

	app, err = ParseAppId("com.ubuntu.clock_clock_10" + ver)
	c.Assert(err, IsNil)
	c.Check(u.Installed(app, false), Equals, false)

	// setVersion
	app, err = ParseAppId("com.ubuntu.clock_clock")
	c.Assert(err, IsNil)
	c.Check(u.Installed(app, true), Equals, true)
	c.Check(app.Version, Equals, ver)
}

func (s *clickSuite) TestInstalledLegacy(c *C) {
	u, err := User()
	c.Assert(err, IsNil)
	app, err := ParseAppId(fmt.Sprintf("_python%s", GetPyVer()))
	c.Assert(err, IsNil)
	c.Check(u.Installed(app, false), Equals, true)
}

func (s *clickSuite) TestParseAndVerifyAppId(c *C) {
	u, err := User()
	c.Assert(err, IsNil)

	app, err := ParseAndVerifyAppId("_.foo", nil)
	c.Assert(err, Equals, ErrInvalidAppId)
	c.Check(app, IsNil)

	app, err = ParseAndVerifyAppId("com.foo.bar_baz", nil)
	c.Assert(err, IsNil)
	c.Check(app.Click, Equals, true)
	c.Check(app.Application, Equals, "baz")

	app, err = ParseAndVerifyAppId("_non-existent-app", u)
	c.Assert(err, Equals, ErrMissingApp)
	c.Check(app, NotNil)
	c.Check(app.Original(), Equals, "_non-existent-app")
}

func (s *clickSuite) TestSymbolicAppendsSymbolicIfIconIsName(c *C) {
	symb := symbolic("foo")
	c.Check(symb, Equals, "foo-symbolic")
}

func (s *clickSuite) TestSymbolicLeavesAloneIfIconIsPath(c *C) {
	symb := symbolic("foo/bar")
	c.Check(symb, Equals, "foo/bar")
}

func (s *clickSuite) TestSymbolicIconCallsSymbolic(c *C) {
	symbolic = func(string) string { return "xyzzy" }
	defer func() { symbolic = _symbolic }()
	app, err := ParseAppId(fmt.Sprintf("_python%s", GetPyVer()))
	c.Assert(err, IsNil)
	c.Check(app.SymbolicIcon(), Equals, "xyzzy")
}

func (s *clickSuite) TestSymbolicFromDesktopFile(c *C) {
	orig := cappinfo.AppSymbolicIconFromDesktopId
	cappinfo.AppSymbolicIconFromDesktopId = func(desktopId string) string {
		return "/foo/symbolic"
	}
	defer func() {
		cappinfo.AppSymbolicIconFromDesktopId = orig
	}()
	app, _ := ParseAppId("com.ubuntu.clock_clock_1.2")
	c.Check(app.SymbolicIcon(), Equals, "/foo/symbolic")
}
