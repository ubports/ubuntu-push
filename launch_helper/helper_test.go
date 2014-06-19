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

package launch_helper

import (
	"testing"

	. "launchpad.net/gocheck"

	helpers "launchpad.net/ubuntu-push/testing"
)

func Test(t *testing.T) { TestingT(t) }

type runnerSuite struct {
	testlog *helpers.TestLogger
}

var _ = Suite(&runnerSuite{})

func (s *runnerSuite) SetUpTest(c *C) {
	s.testlog = helpers.NewTestLogger(c, "error")
}

var runnerTests = []struct {
	expected ReturnValue                                                        // expected result
	msg      string                                                             // description of failure
	starter  func(*_Ctype_gchar, *_Ctype_gchar, **_Ctype_gchar) _Ctype_gboolean // starter fake
	stopper  func(*_Ctype_gchar, *_Ctype_gchar) _Ctype_gboolean                 // stopper fake
}{
	{HelperStopped, "Long running helper is not stopped", fakeStartLongLivedHelper, fakeStop},
	{HelperFinished, "Short running helper doesn't finish", fakeStartShortLivedHelper, fakeStop},
	{HelperFailed, "Filure to start helper doesn't fail", fakeStartFailure, fakeStop},
	{HelperFailed, "Error in start argument casting", fakeStartCheckCasting, fakeStop},
	{StopFailed, "Error in stop argument casting", fakeStartLongLivedHelper, fakeStopCheckCasting},
}

func (s *runnerSuite) TestRunner(c *C) {
	for _, tt := range runnerTests {
		StartHelper = tt.starter
		StopHelper = tt.stopper
		runner := New(s.testlog, "foobar")
		command := []string{"foo1", "bar1", "bat1", "baz1"}
		c.Check(runner.Run(command), Equals, tt.expected, Commentf(tt.msg))
	}
}
