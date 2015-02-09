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

package urldispatcher

import (
	"errors"

	. "launchpad.net/gocheck"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	helpers "launchpad.net/ubuntu-push/testing"
	"testing"
)

// hook up gocheck
func TestUrldispatcher(t *testing.T) { TestingT(t) }

type UDSuite struct {
	log          *helpers.TestLogger
	cDispatchURL func(string, string) error
	cTestURL     func([]string) []string
}

var _ = Suite(&UDSuite{})

func (s *UDSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
	s.cDispatchURL = cDispatchURL
	s.cTestURL = cTestURL
	// replace it with a always succeed version
	cDispatchURL = func(url string, appId string) error {
		return nil
	}
}

func (s *UDSuite) TearDownTest(c *C) {
	cDispatchURL = s.cDispatchURL
	cTestURL = s.cTestURL
}

func (s *UDSuite) TestDispatchURLWorks(c *C) {
	ud := New(s.log)
	appId := clickhelp.MustParseAppId("com.example.test_app_0.99")
	err := ud.DispatchURL("this", appId)
	c.Check(err, IsNil)
}

func (s *UDSuite) TestDispatchURLFailsIfCallFails(c *C) {
	cDispatchURL = func(url string, appId string) error {
		return errors.New("fail!")
	}
	ud := New(s.log)
	appId := clickhelp.MustParseAppId("com.example.test_app_0.99")
	err := ud.DispatchURL("this", appId)
	c.Check(err, NotNil)
}

func (s *UDSuite) TestTestURLWorks(c *C) {
	cTestURL = func(url []string) []string {
		return []string{"com.example.test_app_0.99"}
	}
	ud := New(s.log)
	appId := clickhelp.MustParseAppId("com.example.test_app_0.99")
	c.Check(ud.TestURL(appId, []string{"this"}), Equals, true)
	c.Check(s.log.Captured(), Matches, `(?sm).*TestURL: \[this\].*`)
}

func (s *UDSuite) TestTestURLFailsIfCallFails(c *C) {
	cTestURL = func(url []string) []string {
		return []string{}
	}
	ud := New(s.log)
	appId := clickhelp.MustParseAppId("com.example.test_app_0.99")
	c.Check(ud.TestURL(appId, []string{"this"}), Equals, false)
}

func (s *UDSuite) TestTestURLMultipleURLs(c *C) {
	cTestURL = func(url []string) []string {
		return []string{"com.example.test_app_0.99", "com.example.test_app_0.99"}
	}
	ud := New(s.log)
	appId := clickhelp.MustParseAppId("com.example.test_app_0.99")
	urls := []string{"potato://test-app", "potato_a://foo"}
	c.Check(ud.TestURL(appId, urls), Equals, true)
	c.Check(s.log.Captured(), Matches, `(?sm).*TestURL: \[potato://test-app potato_a://foo\].*`)
}

func (s *UDSuite) TestTestURLWrongApp(c *C) {
	cTestURL = func(url []string) []string {
		return []string{"com.example.test_test-app_0.1"}
	}
	ud := New(s.log)
	appId := clickhelp.MustParseAppId("com.example.test_app_0.99")
	urls := []string{"potato://test-app"}
	c.Check(ud.TestURL(appId, urls), Equals, false)
	c.Check(s.log.Captured(), Matches, `(?smi).*notification skipped because of different appid for actions: \[potato://test-app\] - com.example.test_test-app_0.1 != com.example.test_app_0.99`)
}

func (s *UDSuite) TestTestURLOneWrongApp(c *C) {
	cTestURL = func(url []string) []string {
		return []string{"com.example.test_test-app_0", "com.example.test_test-app1"}
	}
	ud := New(s.log)
	appId := clickhelp.MustParseAppId("com.example.test_test-app_0")
	urls := []string{"potato://test-app", "potato_a://foo"}
	c.Check(ud.TestURL(appId, urls), Equals, false)
	c.Check(s.log.Captured(), Matches, `(?smi).*notification skipped because of different appid for actions: \[potato://test-app potato_a://foo\] - com.example.test_test-app1 != com.example.test_test-app.*`)
}

func (s *UDSuite) TestTestURLInvalidURL(c *C) {
	cTestURL = func(url []string) []string {
		return []string{}
	}
	ud := New(s.log)
	appId := clickhelp.MustParseAppId("com.example.test_app_0.2")
	urls := []string{"notsupported://test-app"}
	c.Check(ud.TestURL(appId, urls), Equals, false)
}

func (s *UDSuite) TestTestURLLegacyApp(c *C) {
	cTestURL = func(url []string) []string {
		return []string{"ubuntu-system-settings"}
	}
	ud := New(s.log)
	appId := clickhelp.MustParseAppId("_ubuntu-system-settings")
	urls := []string{"settings://test-app"}
	c.Check(ud.TestURL(appId, urls), Equals, true)
	c.Check(s.log.Captured(), Matches, `(?sm).*TestURL: \[settings://test-app\].*`)
}
