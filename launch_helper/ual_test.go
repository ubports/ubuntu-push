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

type poolSuite struct {
	oldNew func(logger.Logger) cual.HelperState
	log    *helpers.TestLogger
}

var _ = Suite(&poolSuite{})

type fakeHelperState struct {
	obs   int
	err   error
	argCh chan [5]string
}

func (fhs *fakeHelperState) InstallObserver(func(string)) error {
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

func newFake(logger.Logger) cual.HelperState {
	return fakeInstance
}

func (s *poolSuite) SetUpTest(c *C) {
	s.oldNew = NewHelperState
	s.log = helpers.NewTestLogger(c, "debug")
	NewHelperState = newFake
	fakeInstance = &fakeHelperState{argCh: make(chan [5]string, 10)}
	xdgCacheHome = c.MkDir
}

func (s *poolSuite) TearDownTest(c *C) {
	NewHelperState = s.oldNew
	xdgCacheHome = xdg.Cache.Home
}

// check that Stop (tries to) remove the observer
func (s *poolSuite) TestStartStopWork(c *C) {
	pool := NewHelperPool(s.log)
	c.Check(fakeInstance.obs, Equals, 0)
	pool.Start()
	c.Check(fakeInstance.obs, Equals, 1)
	pool.Stop()
	c.Check(fakeInstance.obs, Equals, 0)
}

func (s *poolSuite) TestRunLaunches(c *C) {
	HelperInfo = func(*click.AppId) (string, string) { return "helpId", "bar" }
	defer func() { HelperInfo = _helperInfo }()
	pool := NewHelperPool(s.log)
	pool.Start()
	defer pool.Stop()
	appId := "com.example.test_test-app"
	app := clickhelp.MustParseAppId(appId)
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	pool.Run("fake", &input)
	select {
	case arg := <-fakeInstance.argCh:
		c.Check(arg[:3], DeepEquals, []string{"Launch", "helpId", "bar"})
	case <-time.After(100 * time.Millisecond):
		c.Fatal("didn't call Launch")
	}
	args := pool.(*kindHelperPool).peekId("0", func(*HelperArgs) {})
	c.Assert(args, NotNil)
	args.Timer.Stop()
	c.Check(args.AppId, Equals, "helpId")
	c.Check(args.Input, Equals, &input)
	c.Check(args.FileIn, NotNil)
	c.Check(args.FileOut, NotNil)
}

func (s *poolSuite) TestGetOutputIfHelperLaunchFail(c *C) {
	// invokes actual _helperInfo which fails with "", ""
	pool := NewHelperPool(s.log)
	ch := pool.Start()
	defer pool.Stop()
	app := clickhelp.MustParseAppId("com.example.test_test-app")
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	pool.Run("fake", &input)
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

func (s *poolSuite) TestRunCantLaunch(c *C) {
	HelperInfo = func(*click.AppId) (string, string) { return "helpId", "bar" }
	defer func() { HelperInfo = _helperInfo }()
	fakeInstance.err = cual.ErrCantLaunch
	pool := NewHelperPool(s.log)
	ch := pool.Start()
	defer pool.Stop()
	appId := "com.example.test_test-app"
	app := clickhelp.MustParseAppId(appId)
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	pool.Run("fake", &input)
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
	c.Check(s.log.Captured(), Equals, "DEBUG using helper helpId (exec: bar) for app com.example.test_test-app\n"+"ERROR unable to launch helper helpId: can't launch helper\n"+"ERROR unable to get helper output; putting payload into message\n")
}

func (s *poolSuite) TestRunLaunchesAndTimeout(c *C) {
	HelperInfo = func(*click.AppId) (string, string) { return "helpId", "bar" }
	defer func() {
		HelperInfo = _helperInfo
	}()
	pool := NewHelperPool(s.log)
	pool.(*kindHelperPool).maxRuntime = 500 * time.Millisecond
	ch := pool.Start()
	defer pool.Stop()
	appId := "com.example.test_test-app"
	app := clickhelp.MustParseAppId(appId)
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	pool.Run("fake", &input)
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
	go pool.(*kindHelperPool).OneDone("0")

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}
	c.Check(res.Message, DeepEquals, input.Payload)
}

func (s *poolSuite) TestOneDoneNop(c *C) {
	pool := NewHelperPool(s.log).(*kindHelperPool)
	pool.OneDone("")
}

func (s *poolSuite) TestOneDoneOnValid(c *C) {
	pool := NewHelperPool(s.log).(*kindHelperPool)
	ch := pool.Start()
	defer pool.Stop()

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
	pool.hmap["1"] = &args

	f, err := os.Create(args.FileOut)
	c.Assert(err, IsNil)
	defer f.Close()
	_, err = f.Write([]byte(`{"notification": {"sound": "hello"}}`))
	c.Assert(err, IsNil)

	go pool.OneDone("1")

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}

	expected := HelperOutput{Notification: &Notification{Sound: "hello"}}
	c.Check(res.HelperOutput, DeepEquals, expected)
	c.Check(pool.hmap, HasLen, 0)
}

func (s *poolSuite) TestOneDoneOnBadFileOut(c *C) {
	pool := NewHelperPool(s.log).(*kindHelperPool)
	ch := pool.Start()
	defer pool.Stop()

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
	pool.hmap["1"] = &args

	go pool.OneDone("1")

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}

	expected := HelperOutput{Message: args.Input.Payload}
	c.Check(res.HelperOutput, DeepEquals, expected)
}

func (s *poolSuite) TestOneDonwOnBadJSONOut(c *C) {
	pool := NewHelperPool(s.log).(*kindHelperPool)
	ch := pool.Start()
	defer pool.Stop()

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
	pool.hmap["1"] = &args

	f, err := os.Create(args.FileOut)
	c.Assert(err, IsNil)
	defer f.Close()
	_, err = f.Write([]byte(`potato`))
	c.Assert(err, IsNil)

	go pool.OneDone("1")

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}

	expected := HelperOutput{Message: args.Input.Payload}
	c.Check(res.HelperOutput, DeepEquals, expected)
}

func (s *poolSuite) TestCreateInputTempFile(c *C) {
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

	pool := NewHelperPool(s.log)
	f1, err := pool.(*kindHelperPool).createInputTempFile(input)
	c.Assert(err, IsNil)
	c.Check(f1, Not(Equals), "")
	f2, err := pool.(*kindHelperPool).createOutputTempFile(input)
	c.Assert(err, IsNil)
	c.Check(f2, Not(Equals), "")
	files, err := ioutil.ReadDir(filepath.Dir(f1))
	c.Check(err, IsNil)
	c.Check(files, HasLen, 2)
}

func (s *poolSuite) TestGetTempFilename(c *C) {
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

func (s *poolSuite) TestGetTempDir(c *C) {
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
