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
	"path"
	"testing"

	"launchpad.net/go-xdg/v0"
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
	inputPath := tmpDir + "/test_helper_input"
	outputPath := tmpDir + "/test_helper_output"
	// start the loop inside a function, with a channel to signal when it's done.
	finished := make(chan bool)
	go func() {
		hr.Start()
		finished <- true
	}()
	msg := []byte("{\"msg\": \"foo\"}")
	ioutil.WriteFile(inputPath, []byte(""), os.ModePerm)
	ioutil.WriteFile(outputPath, msg, os.ModePerm)
	helperArgs := HelperArgs{"bar1", msg, inputPath, outputPath}
	hr.Helpers <- helperArgs
	result := <-hr.Results
	// check the result
	expected := RunnerResult{HelperFinished, helperArgs, msg, nil}
	c.Check(result, DeepEquals, expected)
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
	inputPath := tmpDir + "/test_helper_input"
	outputPath := tmpDir + "/test_helper_output"
	// start the loop inside a function, with a channel to signal when it's done.
	finished := make(chan bool)
	go func() {
		hr.Start()
		finished <- true
	}()
	msg := []byte("{\"msg\": \"foo\"}")
	ioutil.WriteFile(inputPath, []byte(""), os.ModePerm)
	ioutil.WriteFile(outputPath, msg, os.ModePerm)
	helperArgs := HelperArgs{"bar1", msg, inputPath, outputPath}
	hr.Helpers <- helperArgs
	result := <-hr.Results
	// check the result
	expected := RunnerResult{HelperFinished, helperArgs, msg, nil}
	c.Check(result, DeepEquals, expected)
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
	inputPath := tmpDir + "/test_helper_input"
	outputPath := tmpDir + "/test_helper_output"
	// start the loop inside a function, with a channel to signal when it's done.
	finished := make(chan bool)
	go func() {
		hr.Start()
		finished <- true
	}()
	msg := []byte("{\"msg\": \"foo\"}")
	ioutil.WriteFile(inputPath, []byte(""), os.ModePerm)
	helperArgs := HelperArgs{"bar1", msg, inputPath, outputPath}
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
	inputPath := tmpDir + "/test_helper_input"
	outputPath := tmpDir + "/test_helper_output"
	// start the loop inside a function, with a channel to signal when it's done.
	finished := make(chan bool)
	go func() {
		hr.Start()
		finished <- true
	}()
	msg := []byte("{\"msg\": \"foo\"}")
	ioutil.WriteFile(inputPath, []byte(""), os.ModePerm)
	helperArgs := HelperArgs{"bar1", msg, inputPath, outputPath}
	hr.Helpers <- helperArgs
	result := <-hr.Results
	// check the result
	expected := RunnerResult{HelperFailed, helperArgs, nil, errors.New("Helper failed.")}
	c.Check(result, DeepEquals, expected)
	close(hr.Helpers)
	<-finished
	files, err := ioutil.ReadDir(tmpDir)
	c.Check(err, IsNil)
	c.Check(files, HasLen, 0)
}

func (s *runnerSuite) TestFailtoCreateFile(c *C) {
	startHelper = fakeStartFailure
	stopHelper = fakeStop
	// restore it when we are done
	getTempFilename = func(pkgName string) (string, error) {
		return "", errors.New("Can't create files.")
	}
	defer func() {
		getTempFilename = _getTempFilename
	}()
	hr := New(s.testlog, "test_helper")
	// start the loop inside a function, with a channel to signal when it's done.
	finished := make(chan bool)
	go func() {
		hr.Start()
		finished <- true
	}()
	helperArgs := HelperArgs{}
	helperArgs.AppId = "bar1"
	helperArgs.Payload = []byte("{\"msg\": \"foo\"}")
	hr.Helpers <- helperArgs
	result := <-hr.Results
	// check the result
	expected := RunnerResult{HelperFailed, helperArgs, nil, errors.New("Can't create files.")}
	c.Check(result, DeepEquals, expected)
	close(hr.Helpers)
	<-finished
}

func (s *runnerSuite) TestCreateTempFiles(c *C) {
	tmpDir := c.MkDir()
	getTempDir = func(pkgName string) (string, error) {
		return tmpDir, nil
	}
	// restore it when we are done
	defer func() {
		getTempDir = _getTempDir
	}()
	helperArgs := HelperArgs{}
	helperArgs.AppId = "bar1"
	helperArgs.Payload = []byte{}
	c.Check(helperArgs.Input, Equals, "")
	c.Check(helperArgs.Output, Equals, "")
	hr := New(s.testlog, "test_helper")
	err := hr.createTempFiles(&helperArgs, helperArgs.AppId)
	c.Check(err, IsNil)
	c.Check(helperArgs.Input, Not(Equals), "")
	c.Check(helperArgs.Output, Not(Equals), "")
	files, err := ioutil.ReadDir(path.Dir(helperArgs.Input))
	c.Check(err, IsNil)
	c.Check(files, HasLen, 2)
}

func (s *runnerSuite) TestCreateTempFilesWithFilenames(c *C) {
	tmpDir := c.MkDir()
	inputPath := tmpDir + "/test_helper_input"
	outputPath := tmpDir + "/test_helper_output"
	helperArgs := HelperArgs{"bar1", []byte{}, inputPath, outputPath}
	c.Check(helperArgs.Input, Equals, inputPath)
	c.Check(helperArgs.Output, Equals, outputPath)
	hr := New(s.testlog, "test_helper")
	err := hr.createTempFiles(&helperArgs, "pkg.name")
	c.Check(err, IsNil)
	files, err := ioutil.ReadDir(tmpDir)
	c.Check(err, IsNil)
	c.Check(files, HasLen, 0)
}

func (s *runnerSuite) TestGetTempFilename(c *C) {
	getTempDir = func(pkgName string) (string, error) {
		return c.MkDir(), nil
	}
	// restore it when we are done
	defer func() {
		getTempDir = _getTempDir
	}()
	fname, err := getTempFilename("pkg.name")
	c.Check(err, IsNil)
	dirname := path.Dir(fname)
	files, err := ioutil.ReadDir(dirname)
	c.Check(err, IsNil)
	c.Check(files, HasLen, 1)
}

func (s *runnerSuite) TestGetTempDir(c *C) {
	tmpDir := c.MkDir()
	xdgCacheHome = func() string {
		return tmpDir
	}
	// restore it when we are done
	defer func() {
		xdgCacheHome = xdg.Cache.Home
	}()
	dname, err := getTempDir("pkg.name")
	c.Check(err, IsNil)
	c.Check(dname, Equals, path.Join(tmpDir, "pkg.name"))
}
