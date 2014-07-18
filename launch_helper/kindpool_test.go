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
	helpers "launchpad.net/ubuntu-push/testing"
)

type poolSuite struct {
	log  *helpers.TestLogger
	pool HelperPool
}

var _ = Suite(&poolSuite{})

type fakeHelperLauncher struct {
	done  func(string)
	obs   int
	err   error
	lhex  string
	argCh chan [5]string
}

func (fhl *fakeHelperLauncher) InstallObserver(done func(string)) error {
	fhl.done = done
	fhl.obs++
	return nil
}

func (fhl *fakeHelperLauncher) RemoveObserver() error {
	fhl.obs--
	return nil
}

func (fhl *fakeHelperLauncher) HelperInfo(app *click.AppId) (string, string) {
	if app.Click {
		return app.Base() + "-helper", "bar"
	} else {
		return "", fhl.lhex
	}
}

func (fhl *fakeHelperLauncher) Launch(appId string, exec string, f1 string, f2 string) (string, error) {
	fhl.argCh <- [5]string{"Launch", appId, exec, f1, f2}
	return "0", fhl.err
}

func (fhl *fakeHelperLauncher) Stop(appId string, iid string) error {
	fhl.argCh <- [5]string{"Stop", appId, iid, "", ""}
	return nil
}

var fakeLauncher *fakeHelperLauncher

func (s *poolSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
	fakeLauncher = &fakeHelperLauncher{argCh: make(chan [5]string, 10)}
	s.pool = NewHelperPool(map[string]HelperLauncher{"fake": fakeLauncher}, s.log)
	xdgCacheHome = c.MkDir
}

func (s *poolSuite) TearDownTest(c *C) {
	s.pool = nil
	xdgCacheHome = xdg.Cache.Home
}

func (s *poolSuite) TestDefaultLaunchers(c *C) {
	launchers := DefaultLaunchers(s.log)
	_, ok := launchers["click"]
	c.Check(ok, Equals, true)
	_, ok = launchers["legacy"]
	c.Check(ok, Equals, true)
}

// check that Stop (tries to) remove the observer
func (s *poolSuite) TestStartStopWork(c *C) {
	c.Check(fakeLauncher.obs, Equals, 0)
	s.pool.Start()
	c.Check(fakeLauncher.done, NotNil)
	c.Check(fakeLauncher.obs, Equals, 1)
	s.pool.Stop()
	c.Check(fakeLauncher.obs, Equals, 0)
}

func (s *poolSuite) TestRunLaunches(c *C) {
	s.pool.Start()
	defer s.pool.Stop()
	appId := "com.example.test_test-app"
	app := clickhelp.MustParseAppId(appId)
	helpId := app.Base() + "-helper"
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	s.pool.Run("fake", &input)
	select {
	case arg := <-fakeLauncher.argCh:
		c.Check(arg[:3], DeepEquals, []string{"Launch", helpId, "bar"})
	case <-time.After(100 * time.Millisecond):
		c.Fatal("didn't call Launch")
	}
	args := s.pool.(*kindHelperPool).peekId("fake:0", func(*HelperArgs) {})
	c.Assert(args, NotNil)
	args.Timer.Stop()
	c.Check(args.AppId, Equals, helpId)
	c.Check(args.Input, Equals, &input)
	c.Check(args.FileIn, NotNil)
	c.Check(args.FileOut, NotNil)
}

func (s *poolSuite) TestRunLaunchesLegacyStyle(c *C) {
	fakeLauncher.lhex = "lhex"
	s.pool.Start()
	defer s.pool.Stop()
	appId := "_legacy"
	app := clickhelp.MustParseAppId(appId)
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	s.pool.Run("fake", &input)
	select {
	case arg := <-fakeLauncher.argCh:
		c.Check(arg[:3], DeepEquals, []string{"Launch", "", "lhex"})
	case <-time.After(100 * time.Millisecond):
		c.Fatal("didn't call Launch")
	}
	args := s.pool.(*kindHelperPool).peekId("fake:0", func(*HelperArgs) {})
	c.Assert(args, NotNil)
	args.Timer.Stop()
	c.Check(args.Input, Equals, &input)
	c.Check(args.FileIn, NotNil)
	c.Check(args.FileOut, NotNil)
}

func (s *poolSuite) TestGetOutputIfHelperLaunchFail(c *C) {
	ch := s.pool.Start()
	defer s.pool.Stop()
	app := clickhelp.MustParseAppId("com.example.test_test-app")
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	s.pool.Run("not-there", &input)
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

func (s *poolSuite) TestGetOutputIfHelperLaunchFail2(c *C) {
	ch := s.pool.Start()
	defer s.pool.Stop()
	app := clickhelp.MustParseAppId("_legacy")
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	s.pool.Run("fake", &input)
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
	fakeLauncher.err = cual.ErrCantLaunch
	ch := s.pool.Start()
	defer s.pool.Stop()
	appId := "com.example.test_test-app"
	app := clickhelp.MustParseAppId(appId)
	helpId := app.Base() + "-helper"
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	s.pool.Run("fake", &input)
	select {
	case arg := <-fakeLauncher.argCh:
		c.Check(arg[:3], DeepEquals, []string{"Launch", helpId, "bar"})
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
	c.Check(s.log.Captured(), Equals, "DEBUG using helper com.example.test_test-app-helper (exec: bar) for app com.example.test_test-app\n"+"ERROR unable to launch helper com.example.test_test-app-helper: can't launch helper\n"+"ERROR unable to get helper output; putting payload into message\n")
}

func (s *poolSuite) TestRunLaunchesAndTimeout(c *C) {
	s.pool.(*kindHelperPool).maxRuntime = 500 * time.Millisecond
	ch := s.pool.Start()
	defer s.pool.Stop()
	appId := "com.example.test_test-app"
	app := clickhelp.MustParseAppId(appId)
	helpId := app.Base() + "-helper"
	input := HelperInput{
		App:            app,
		NotificationId: "foo",
		Payload:        []byte(`"hello"`),
	}
	s.pool.Run("fake", &input)
	select {
	case arg := <-fakeLauncher.argCh:
		c.Check(arg[0], Equals, "Launch")
	case <-time.After(100 * time.Millisecond):
		c.Fatal("didn't call Launch")
	}
	select {
	case arg := <-fakeLauncher.argCh:
		c.Check(arg[:3], DeepEquals, []string{"Stop", helpId, "0"})
	case <-time.After(2 * time.Second):
		c.Fatal("didn't call Stop")
	}
	// this will be invoked
	go fakeLauncher.done("0")

	var res *HelperResult
	select {
	case res = <-ch:
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout")
	}
	c.Check(res.Message, DeepEquals, input.Payload)
}

func (s *poolSuite) TestOneDoneNop(c *C) {
	pool := s.pool.(*kindHelperPool)
	pool.OneDone("")
}

func (s *poolSuite) TestOneDoneOnValid(c *C) {
	pool := s.pool.(*kindHelperPool)
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
	pool.hmap["l:1"] = &args

	f, err := os.Create(args.FileOut)
	c.Assert(err, IsNil)
	defer f.Close()
	_, err = f.Write([]byte(`{"notification": {"sound": "hello"}}`))
	c.Assert(err, IsNil)

	go pool.OneDone("l:1")

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
	pool := s.pool.(*kindHelperPool)
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
	pool.hmap["l:1"] = &args

	go pool.OneDone("l:1")

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
	pool := s.pool.(*kindHelperPool)
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
	pool.hmap["l:1"] = &args

	f, err := os.Create(args.FileOut)
	c.Assert(err, IsNil)
	defer f.Close()
	_, err = f.Write([]byte(`potato`))
	c.Assert(err, IsNil)

	go pool.OneDone("l:1")

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

	pool := s.pool.(*kindHelperPool)
	f1, err := pool.createInputTempFile(input)
	c.Assert(err, IsNil)
	c.Check(f1, Not(Equals), "")
	f2, err := pool.createOutputTempFile(input)
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
