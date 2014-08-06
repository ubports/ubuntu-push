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

package service

import (
	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/click"
)

type commonSuite struct{}

var _ = Suite(&commonSuite{})

func (cs *commonSuite) TestGrabDBusPackageAndAppIdWorks(c *C) {
	svc := new(DBusService)
	aDBusPath := "/com/ubuntu/Postal/com_2eexample_2etest"
	aPackage := "com.example.test"
	anAppId := aPackage + "_test"
	app, err := svc.grabDBusPackageAndAppId(aDBusPath, []interface{}{anAppId}, 0)
	c.Check(err, IsNil)
	c.Check(app.Package, Equals, aPackage)
	c.Check(app.Original(), Equals, anAppId)
}

type fakeInstalledChecker struct{}

func (fakeInstalledChecker) Installed(app *click.AppId, setVersion bool) bool {
	return app.Original()[0] == 'c'
}

func (cs *commonSuite) TestGrabDBusPackageAndAppIdFails(c *C) {
	svc := new(DBusService)
	svc.installedChecker = fakeInstalledChecker{}
	aDBusPath := "/com/ubuntu/Postal/com_2eexample_2etest"
	aPackage := "com.example.test"
	anAppId := aPackage + "_test"

	for i, s := range []struct {
		path     string
		args     []interface{}
		numExtra int
		errt     error
	}{
		{aDBusPath, []interface{}{}, 0, ErrBadArgCount},
		{aDBusPath, []interface{}{anAppId}, 1, ErrBadArgCount},
		{aDBusPath, []interface{}{anAppId, anAppId}, 0, ErrBadArgCount},
		{aDBusPath, []interface{}{1}, 0, ErrBadArgType},
		{aDBusPath, []interface{}{aPackage}, 0, click.ErrInvalidAppId},
		{aDBusPath, []interface{}{"x" + anAppId}, 0, click.ErrMissingApp},
		{aDBusPath, []interface{}{"c" + anAppId}, 0, ErrBadAppId},
	} {
		comment := Commentf("iteration #%d", i)
		app, err := svc.grabDBusPackageAndAppId(s.path, s.args, s.numExtra)
		c.Check(err, Equals, s.errt, comment)
		c.Check(app, IsNil, comment)
	}
}
