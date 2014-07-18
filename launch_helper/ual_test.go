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
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"launchpad.net/go-xdg/v0"
	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/click"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	"launchpad.net/ubuntu-push/launch_helper/cual"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
)

type ualSuite struct {
	oldNew func(logger.Logger, cual.UAL) cual.HelperState
	log    *helpers.TestLogger
}

var _ = Suite(&ualSuite{})

type fakeHelperState struct {
	obs   int
	err   error
	argCh chan [5]string
}

func (fhs *fakeHelperState) InstallObserver() error {
	fhs.obs++
	return nil
}

func (fhs *fakeHelperState) RemoveObserver() error {
	fhs.obs--
	return nil
}

func (fhs *fakeHelperState) Launch(appId string, exec string, f1 string, f2 string) (string, error) {
	fhs.argCh <- [5]string{"Launch", appId, exec, f1, f2}
	return "0", fhs.err
}

func (fhs *fakeHelperState) Stop(appId string, iid string) error {
	fhs.argCh <- [5]string{"Stop", appId, iid, "", ""}
	return nil
}

var fakeInstance *fakeHelperState

func newFake(logger.Logger, cual.UAL) cual.HelperState {
	return fakeInstance
}

func (us *ualSuite) SetUpTest(c *C) {
	us.oldNew = NewHelperState
	us.log = helpers.NewTestLogger(c, "debug")
	NewHelperState = newFake
	fakeInstance = &fakeHelperState{argCh: make(chan [5]string, 10)}
	xdgCacheHome = c.MkDir
}

func (us *ualSuite) TearDownTest(c *C) {
	NewHelperState = us.oldNew
	xdgCacheHome = xdg.Cache.Home
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
	HelperInfo = func(*click.AppId) (string, string) { return "helpId", "bar" }
	defer func() { HelperInfo = _helperInfo }()
	ual := NewHelperLauncher(us.log)
	ual.Start()
	defer ual.Stop()
	appId := "com.example.test_test-app"
	app := clickhelp.MustParseAppId(appId)
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	ual.Run(&input)
	select {
	case arg := <-fakeInstance.argCh:
		c.Check(arg[:3], DeepEquals, []string{"Launch", "helpId", "bar"})
	case <-time.After(100 * time.Millisecond):
		c.Fatal("didn't call Launch")
	}
	args := ual.(*ualHelperLauncher).peekId("0", func(*HelperArgs) {})
	c.Assert(args, NotNil)
	args.Timer.Stop()
	c.Check(args.AppId, Equals, "helpId")
	c.Check(args.Input, Equals, &input)
	c.Check(args.FileIn, NotNil)
	c.Check(args.FileOut, NotNil)
}

func (us *ualSuite) TestGetOutputIfHelperLaunchFail(c *C) {
	// invokes actual _helperInfo which fails with "", ""
	ual := NewHelperLauncher(us.log)
	ch := ual.Start()
	defer ual.Stop()
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

func (us *ualSuite) TestRunCantLaunch(c *C) {
	HelperInfo = func(*click.AppId) (string, string) { return "helpId", "bar" }
	defer func() { HelperInfo = _helperInfo }()
	fakeInstance.err = cual.ErrCantLaunch
	ual := NewHelperLauncher(us.log)
	ch := ual.Start()
	defer ual.Stop()
	appId := "com.example.test_test-app"
	app := clickhelp.MustParseAppId(appId)
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	ual.Run(&input)
	select {
	case arg := <-fakeInstance.argCh:
		c.Check(arg[:3], DeepEquals, []string{"Launch", "helpId", "bar"})
	case <-time.After(100 * time.Millisecond):
		c.Fatal("didn't call Launch")
	}
	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}
	c.Check(res.Message, DeepEquals, input.Payload)
	c.Check(us.log.Captured(), Equals, "DEBUG using helper helpId (exec: bar) for app com.example.test_test-app\n"+"ERROR unable to launch helper helpId: can't launch helper\n"+"ERROR unable to get helper output; putting payload into message\n")
}

func (us *ualSuite) TestRunLaunchesAndTimeout(c *C) {
	HelperInfo = func(*click.AppId) (string, string) { return "helpId", "bar" }
	defer func() {
		HelperInfo = _helperInfo
	}()
	ual := NewHelperLauncher(us.log)
	ual.(*ualHelperLauncher).maxRuntime = 500 * time.Millisecond
	ch := ual.Start()
	defer ual.Stop()
	appId := "com.example.test_test-app"
	app := clickhelp.MustParseAppId(appId)
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	ual.Run(&input)
	select {
	case arg := <-fakeInstance.argCh:
		c.Check(arg[0], Equals, "Launch")
	case <-time.After(100 * time.Millisecond):
		c.Fatal("didn't call Launch")
	}
	select {
	case arg := <-fakeInstance.argCh:
		c.Check(arg[:3], DeepEquals, []string{"Stop", "helpId", "0"})
	case <-time.After(2 * time.Second):
		c.Fatal("didn't call Stop")
	}
	// this will be invoked
	go ual.(*ualHelperLauncher).OneDone("0")

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}
	c.Check(res.Message, DeepEquals, input.Payload)
}

func (us *ualSuite) TestOneDoneNop(c *C) {
	ual := NewHelperLauncher(us.log).(*ualHelperLauncher)
	ual.OneDone("")
}

func (us *ualSuite) TestOneDoneOnValid(c *C) {
	ual := NewHelperLauncher(us.log).(*ualHelperLauncher)
	ch := ual.Start()
	defer ual.Stop()

	d := c.MkDir()

	app := clickhelp.MustParseAppId("com.example.test_test-app")
	input := &HelperInput{
		App: app,
	}
	args := HelperArgs{
		Input:   input,
		FileOut: filepath.Join(d, "file_out.json"),
		Timer:   &time.Timer{},
	}
	ual.hmap["1"] = &args

	f, err := os.Create(args.FileOut)
	c.Assert(err, IsNil)
	defer f.Close()
	_, err = f.Write([]byte(`{"notification": {"sound": "hello"}}`))
	c.Assert(err, IsNil)

	go ual.OneDone("1")

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}

	expected := HelperOutput{Notification: &Notification{Sound: "hello"}}
	c.Check(res.HelperOutput, DeepEquals, expected)
	c.Check(ual.hmap, HasLen, 0)
}

func (us *ualSuite) TestOneDoneOnBadFileOut(c *C) {
	ual := NewHelperLauncher(us.log).(*ualHelperLauncher)
	ch := ual.Start()
	defer ual.Stop()

	app := clickhelp.MustParseAppId("com.example.test_test-app")
	args := HelperArgs{
		Input: &HelperInput{
			App:            app,
			NotificationId: "foo",
			Payload:        []byte(`"hello"`),
		},
		FileOut: "/does-not-exist",
		Timer:   &time.Timer{},
	}
	ual.hmap["1"] = &args

	go ual.OneDone("1")

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}

	expected := HelperOutput{Message: args.Input.Payload}
	c.Check(res.HelperOutput, DeepEquals, expected)
}

func (us *ualSuite) TestOneDonwOnBadJSONOut(c *C) {
	ual := NewHelperLauncher(us.log).(*ualHelperLauncher)
	ch := ual.Start()
	defer ual.Stop()

	d := c.MkDir()

	app := clickhelp.MustParseAppId("com.example.test_test-app")
	args := HelperArgs{
		FileOut: filepath.Join(d, "file_out.json"),
		Input: &HelperInput{
			App:            app,
			NotificationId: "foo",
			Payload:        []byte(`"hello"`),
		},
		Timer: &time.Timer{},
	}
	ual.hmap["1"] = &args

	f, err := os.Create(args.FileOut)
	c.Assert(err, IsNil)
	defer f.Close()
	_, err = f.Write([]byte(`potato`))
	c.Assert(err, IsNil)

	go ual.OneDone("1")

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}

	expected := HelperOutput{Message: args.Input.Payload}
	c.Check(res.HelperOutput, DeepEquals, expected)
}

func (us *ualSuite) TestCreateInputTempFile(c *C) {
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
	f1, err := ual.(*ualHelperLauncher).createInputTempFile(input)
	c.Assert(err, IsNil)
	c.Check(f1, Not(Equals), "")
	f2, err := ual.(*ualHelperLauncher).createOutputTempFile(input)
	c.Assert(err, IsNil)
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
