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
	"errors"
	"io/ioutil"
	"os"
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

func (s *runnerSuite) TestARunner(c *C) {
	for _, tt := range runnerTests {
		startHelper = tt.starter
		stopHelper = tt.stopper
		runner := New(s.testlog, "foo1")
		result := runner.Run("bar1", "bat1", "baz1")
		c.Check(result, Equals, tt.expected, Commentf(tt.msg))
	}
}

func (s *runnerSuite) TestFileHandling(c *C) {
	startHelper = fakeStartShortLivedHelper
	stopHelper = fakeStop
	hr := New(s.testlog, "test_helper")
	tmpDir := c.MkDir()
	input_path := tmpDir + "/test_helper_input"
	output_path := tmpDir + "/test_helper_output"
	// start the loop inside a function, with a channel to signal when it's done.
	finished := make(chan bool)
	go func() {
		hr.Start()
		finished <- true
	}()
	msg := []byte("{\"msg\": \"foo\"}")
	ioutil.WriteFile(input_path, []byte(""), os.ModePerm)
	ioutil.WriteFile(output_path, msg, os.ModePerm)
	helperArgs := HelperArgs{"bar1", msg, input_path, output_path}
	hr.Helpers <- helperArgs
	result := <-hr.Results
	// check the result
	expected := RunnerResult{HelperFinished, helperArgs, msg, nil}
	c.Check(result.Status, Equals, expected.Status)
	c.Check(result.Helper, DeepEquals, expected.Helper)
	c.Check(string(result.Data), Equals, string(expected.Data))
	c.Check(result.Error, Equals, expected.Error)
	close(hr.Helpers)
	<-finished
	files, err := ioutil.ReadDir(tmpDir)
	c.Check(err, IsNil)
	c.Check(files, HasLen, 0)
}

func (s *runnerSuite) TestFileHandlingLongRunningHelperOK(c *C) {
	startHelper = fakeStartLongLivedHelper
	stopHelper = fakeStop
	hr := New(s.testlog, "test_helper")
	tmpDir := c.MkDir()
	input_path := tmpDir + "/test_helper_input"
	output_path := tmpDir + "/test_helper_output"
	// start the loop inside a function, with a channel to signal when it's done.
	finished := make(chan bool)
	go func() {
		hr.Start()
		finished <- true
	}()
	msg := []byte("{\"msg\": \"foo\"}")
	ioutil.WriteFile(input_path, []byte(""), os.ModePerm)
	ioutil.WriteFile(output_path, msg, os.ModePerm)
	helperArgs := HelperArgs{"bar1", msg, input_path, output_path}
	hr.Helpers <- helperArgs
	result := <-hr.Results
	// check the result
	expected := RunnerResult{HelperFinished, helperArgs, msg, nil}
	c.Check(result.Status, Equals, expected.Status)
	c.Check(result.Helper, DeepEquals, expected.Helper)
	c.Check(string(result.Data), Equals, string(expected.Data))
	c.Check(result.Error, Equals, expected.Error)
	close(hr.Helpers)
	<-finished
	files, err := ioutil.ReadDir(tmpDir)
	c.Check(err, IsNil)
	c.Check(files, HasLen, 0)
}

func (s *runnerSuite) TestFileHandlingLongRunningHelperNoOutput(c *C) {
	startHelper = fakeStartLongLivedHelper
	stopHelper = fakeStop
	hr := New(s.testlog, "test_helper")
	tmpDir := c.MkDir()
	input_path := tmpDir + "/test_helper_input"
	output_path := tmpDir + "/test_helper_output"
	// start the loop inside a function, with a channel to signal when it's done.
	finished := make(chan bool)
	go func() {
		hr.Start()
		finished <- true
	}()
	msg := []byte("{\"msg\": \"foo\"}")
	ioutil.WriteFile(input_path, []byte(""), os.ModePerm)
	helperArgs := HelperArgs{"bar1", msg, input_path, output_path}
	hr.Helpers <- helperArgs
	result := <-hr.Results
	// check the result
	expected := RunnerResult{HelperFailed, helperArgs, msg, nil}
	c.Check(result.Status, Equals, expected.Status)
	c.Check(result.Helper, DeepEquals, expected.Helper)
	c.Check(string(result.Data), Equals, "")
	c.Check(result.Error, ErrorMatches, ".*no such file.*")
	close(hr.Helpers)
	<-finished
	files, err := ioutil.ReadDir(tmpDir)
	c.Check(err, IsNil)
	c.Check(files, HasLen, 0)
}

func (s *runnerSuite) TestFileHandlingFailed(c *C) {
	startHelper = fakeStartFailure
	stopHelper = fakeStop
	hr := New(s.testlog, "test_helper")
	tmpDir := c.MkDir()
	input_path := tmpDir + "/test_helper_input"
	output_path := tmpDir + "/test_helper_output"
	// start the loop inside a function, with a channel to signal when it's done.
	finished := make(chan bool)
	go func() {
		hr.Start()
		finished <- true
	}()
	msg := []byte("{\"msg\": \"foo\"}")
	ioutil.WriteFile(input_path, []byte(""), os.ModePerm)
	helperArgs := HelperArgs{"bar1", msg, input_path, output_path}
	hr.Helpers <- helperArgs
	result := <-hr.Results
	// check the result
	expected := RunnerResult{HelperFailed, helperArgs, nil, errors.New("Helper failed.")}
	c.Check(result.Status, Equals, expected.Status)
	c.Check(result.Helper, DeepEquals, expected.Helper)
	c.Check(string(result.Data), Equals, "")
	c.Check(result.Error.Error(), Equals, expected.Error.Error())
	close(hr.Helpers)
	<-finished
	files, err := ioutil.ReadDir(tmpDir)
	c.Check(err, IsNil)
	c.Check(files, HasLen, 0)
}
