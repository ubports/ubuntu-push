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
)

type commonSuite struct{}

var _ = Suite(&commonSuite{})

func (cs *commonSuite) TestGrabDBusPackageAndAppIdWorks(c *C) {
	aDBusPath := "/com/ubuntu/Postal/com_2eexample_2etest"
	aPackage := "com.example.test"
	anAppId := aPackage + "_test"
	pkg, app, err := grabDBusPackageAndAppId(aDBusPath, []interface{}{anAppId}, 0)
	c.Check(err, IsNil)
	c.Check(pkg, Equals, aPackage)
	c.Check(app, Equals, anAppId)
}

func (cs *commonSuite) TestGrabDBusPackageAndAppIdFails(c *C) {
	aDBusPath := "/com/ubuntu/Postal/com_2eexample_2etest"
	aPackage := "com.example.test"
	anAppId := aPackage + "_test"

	for i, s := range []struct {
		path     string
		args     []interface{}
		numExtra int
		errt     error
	}{
		{aDBusPath, []interface{}{}, 0, BadArgCount},
		{aDBusPath, []interface{}{anAppId}, 1, BadArgCount},
		{aDBusPath, []interface{}{anAppId, anAppId}, 0, BadArgCount},
		{aDBusPath, []interface{}{1}, 0, BadArgType},
		{aDBusPath, []interface{}{aPackage}, 0, BadAppId},
	} {
		comment := Commentf("iteration #%d", i)
		pkg, app, err := grabDBusPackageAndAppId(s.path, s.args, s.numExtra)
		c.Check(err, Equals, s.errt, comment)
		c.Check(pkg, Equals, "", comment)
		c.Check(app, Equals, "", comment)
	}
}
