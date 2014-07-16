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
	"path/filepath"
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/launch_helper/cual"
	"launchpad.net/ubuntu-push/logger"
	// "launchpad.net/ubuntu-push/click"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	helpers "launchpad.net/ubuntu-push/testing"
)

type ualSuite struct {
	oldNew func(logger.Logger) cual.HelperState
	log    *helpers.TestLogger
}

var _ = Suite(&ualSuite{})

type fakeHelperState struct {
	obs   int
	err   error
	argCh chan *cual.HelperArgs
}

func (fhs *fakeHelperState) InstallObserver() error {
	fhs.obs++
	return fhs.err
}

func (fhs *fakeHelperState) RemoveObserver() error {
	fhs.obs--
	return fhs.err
}

func (fhs *fakeHelperState) Launch(args *cual.HelperArgs) error {
	fhs.argCh <- args
	return fhs.err
}

var fakeInstance *fakeHelperState

func newFake(log logger.Logger) cual.HelperState {
	return fakeInstance
}

func (us *ualSuite) SetUpTest(c *C) {
	us.oldNew = newHelperState
	us.log = helpers.NewTestLogger(c, "debug")
	newHelperState = newFake
	fakeInstance = &fakeHelperState{argCh: make(chan *cual.HelperArgs, 10)}
}

func (us *ualSuite) TearDownTest(c *C) {
	newHelperState = us.oldNew
}

// check that Stop (tries to) remove the observer
func (us *ualSuite) TestStartStopWork(c *C) {
	ual := NewHelperLauncher(us.log)
	c.Check(fakeInstance.obs, Equals, 0)
	ual.Start()
	c.Check(fakeInstance.obs, Equals, 1)
	ual.Stop()
	c.Check(fakeInstance.obs, Equals, 0)
}

func (us *ualSuite) TestRunLaunches(c *C) {
	ual := NewHelperLauncher(us.log)
	ual.Start()
	app := clickhelp.MustParseAppId("com.example.test_test-app")
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	ual.Run(&input)
	select {
	case arg := <-fakeInstance.argCh:
		c.Check(arg.App, Equals, input.App)
		c.Check(arg.NotificationId, Equals, input.NotificationId)
		c.Check(arg.Payload, DeepEquals, input.Payload)
		c.Check(arg.FileIn, Not(Equals), "")
		c.Check(arg.FileOut, Not(Equals), "")
		c.Check(arg.OneDone, NotNil)
	case <-time.After(100 * time.Millisecond):
		c.Fatal("didn't call Launch")
	}
}

func (us *ualSuite) TestGetOutputIfHelperLaunchFail(c *C) {
	fakeInstance.err = errors.New("potato")
	ual := NewHelperLauncher(us.log)
	ch := ual.Start()
	app := clickhelp.MustParseAppId("com.example.test_test-app")
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	ual.Run(&input)
	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}
	c.Check(res.Message, DeepEquals, input.Payload)
	c.Check(res.Notification, IsNil)
	c.Check(*res.Input, DeepEquals, input)
}

func (us *ualSuite) TestOneDonwOnValid(c *C) {
	ual := NewHelperLauncher(us.log)
	ch := ual.Start()

	d := c.MkDir()

	app := clickhelp.MustParseAppId("com.example.test_test-app")
	args := cual.HelperArgs{
		App:            app,
		NotificationId: "foo",
		FileOut:        filepath.Join(d, "file_out.json"),
	}

	f, err := os.Create(args.FileOut)
	c.Assert(err, IsNil)
	defer f.Close()
	_, err = f.Write([]byte(`{"notification": {"sound": "hello"}}`))
	c.Assert(err, IsNil)

	go ual.(*ualHelperLauncher).OneDone(&args)

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}

	expected := HelperOutput{Notification: &Notification{Sound: "hello"}}
	c.Check(res.HelperOutput, DeepEquals, expected)
}

func (us *ualSuite) TestOneDonwOnBadFileOut(c *C) {
	ual := NewHelperLauncher(us.log)
	ch := ual.Start()

	app := clickhelp.MustParseAppId("com.example.test_test-app")
	args := cual.HelperArgs{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
		FileOut:        "/does-not-exist",
	}

	go ual.(*ualHelperLauncher).OneDone(&args)

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}

	expected := HelperOutput{Message: args.Payload}
	c.Check(res.HelperOutput, DeepEquals, expected)
}

func (us *ualSuite) TestOneDonwOnBadJSONOut(c *C) {
	ual := NewHelperLauncher(us.log)
	ch := ual.Start()

	d := c.MkDir()

	app := clickhelp.MustParseAppId("com.example.test_test-app")
	args := cual.HelperArgs{
		App:            app,
		NotificationId: "foo",
		FileOut:        filepath.Join(d, "file_out.json"),
	}

	f, err := os.Create(args.FileOut)
	c.Assert(err, IsNil)
	defer f.Close()
	_, err = f.Write([]byte(`potato`))
	c.Assert(err, IsNil)

	go ual.(*ualHelperLauncher).OneDone(&args)

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}

	expected := HelperOutput{Message: args.Payload}
	c.Check(res.HelperOutput, DeepEquals, expected)
}

func (us *ualSuite) TestCreateTempFiles(c *C) {
	tmpDir := c.MkDir()
	getTempDir = func(pkgName string) (string, error) {
		return tmpDir, nil
	}
	// restore it when we are done
	defer func() {
		getTempDir = _getTempDir
	}()

	app := clickhelp.MustParseAppId("com.example.test_test-app")
	input := &HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}

	ual := NewHelperLauncher(us.log)
	f1, f2, err := ual.(*ualHelperLauncher).createTempFiles(input)
	c.Check(err, IsNil)
	c.Check(f1, Not(Equals), "")
	c.Check(f2, Not(Equals), "")
	files, err := ioutil.ReadDir(filepath.Dir(f1))
	c.Check(err, IsNil)
	c.Check(files, HasLen, 2)
}

func (us *ualSuite) TestGetTempFilename(c *C) {
	getTempDir = func(pkgName string) (string, error) {
		return c.MkDir(), nil
	}
	// restore it when we are done
	defer func() {
		getTempDir = _getTempDir
	}()
	fname, err := getTempFilename("pkg.name")
	c.Check(err, IsNil)
	dirname := filepath.Dir(fname)
	files, err := ioutil.ReadDir(dirname)
	c.Check(err, IsNil)
	c.Check(files, HasLen, 1)
}

func (us *ualSuite) TestGetTempDir(c *C) {
	tmpDir := c.MkDir()
	oldCacheHome := xdgCacheHome
	xdgCacheHome = func() string {
		return tmpDir
	}
	// restore it when we are done
	defer func() {
		xdgCacheHome = oldCacheHome
	}()
	dname, err := getTempDir("pkg.name")
	c.Check(err, IsNil)
	c.Check(dname, Equals, filepath.Join(tmpDir, "pkg.name"))
}
