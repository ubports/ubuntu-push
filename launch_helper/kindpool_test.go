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
	"fmt"
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
	log          *helpers.TestLogger
	pool         HelperPool
	fakeLauncher *fakeHelperLauncher
}

var _ = Suite(&poolSuite{})

func takeNext(ch chan *HelperResult, c *C) *HelperResult {
	select {
	case res := <-ch:
		return res
	case <-time.After(100 * time.Millisecond):
		c.Fatal("timeout waiting for result")
	}
	return nil
}

type fakeHelperLauncher struct {
	done  func(string)
	obs   int
	err   error
	lhex  string
	argCh chan [5]string
	runid int
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
	runid := fmt.Sprintf("%d", fhl.runid)
	fhl.runid++
	return runid, fhl.err
}

func (fhl *fakeHelperLauncher) Stop(appId string, iid string) error {
	fhl.argCh <- [5]string{"Stop", appId, iid, "", ""}
	return nil
}

func (s *poolSuite) waitForArgs(c *C, method string) [5]string {
	var args [5]string
	select {
	case args = <-s.fakeLauncher.argCh:
	case <-time.After(2 * time.Second):
		c.Fatal("didn't call " + method)
	}
	c.Assert(args[0], Equals, method)
	return args
}

func (s *poolSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
	s.fakeLauncher = &fakeHelperLauncher{argCh: make(chan [5]string, 10)}
	s.pool = NewHelperPool(map[string]HelperLauncher{"fake": s.fakeLauncher}, s.log)
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
	c.Check(s.fakeLauncher.obs, Equals, 0)
	s.pool.Start()
	c.Check(s.fakeLauncher.done, NotNil)
	c.Check(s.fakeLauncher.obs, Equals, 1)
	s.pool.Stop()
	c.Check(s.fakeLauncher.obs, Equals, 0)
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
	launchArgs := s.waitForArgs(c, "Launch")
	c.Check(launchArgs[:3], DeepEquals, []string{"Launch", helpId, "bar"})
	args := s.pool.(*kindHelperPool).peekId("fake:0", func(*HelperArgs) {})
	c.Assert(args, NotNil)
	args.Timer.Stop()
	c.Check(args.AppId, Equals, helpId)
	c.Check(args.Input, Equals, &input)
	c.Check(args.FileIn, NotNil)
	c.Check(args.FileOut, NotNil)
}

func (s *poolSuite) TestRunLaunchesLegacyStyle(c *C) {
	s.fakeLauncher.lhex = "lhex"
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
	launchArgs := s.waitForArgs(c, "Launch")
	c.Check(launchArgs[:3], DeepEquals, []string{"Launch", "", "lhex"})
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
	res := takeNext(ch, c)
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
	res := takeNext(ch, c)
	c.Check(res.Message, DeepEquals, input.Payload)
	c.Check(res.Notification, IsNil)
	c.Check(*res.Input, DeepEquals, input)
}

func (s *poolSuite) TestRunCantLaunch(c *C) {
	s.fakeLauncher.err = cual.ErrCantLaunch
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
	launchArgs := s.waitForArgs(c, "Launch")
	c.Check(launchArgs[:3], DeepEquals, []string{"Launch", helpId, "bar"})
	res := takeNext(ch, c)
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
	launchArgs := s.waitForArgs(c, "Launch")
	c.Check(launchArgs[0], Equals, "Launch")
	stopArgs := s.waitForArgs(c, "Stop")
	c.Check(stopArgs[:3], DeepEquals, []string{"Stop", helpId, "0"})
	// this will be invoked
	go s.fakeLauncher.done("0")

	res := takeNext(ch, c)
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
	_, err = f.Write([]byte(`{"notification": {"sound": "hello", "tag": "a-tag"}}`))
	c.Assert(err, IsNil)

	go pool.OneDone("l:1")

	res := takeNext(ch, c)

	expected := HelperOutput{Notification: &Notification{Sound: "hello", Tag: "a-tag"}}
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

	res := takeNext(ch, c)

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

	res := takeNext(ch, c)

	expected := HelperOutput{Message: args.Input.Payload}
	c.Check(res.HelperOutput, DeepEquals, expected)
}

func (s *poolSuite) TestCreateInputTempFile(c *C) {
	tmpDir := c.MkDir()
	GetTempDir = func(pkgName string) (string, error) {
		return tmpDir, nil
	}
	// restore it when we are done
	defer func() {
		GetTempDir = _getTempDir
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
	GetTempDir = func(pkgName string) (string, error) {
		return c.MkDir(), nil
	}
	// restore it when we are done
	defer func() {
		GetTempDir = _getTempDir
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
	dname, err := GetTempDir("pkg.name")
	c.Check(err, IsNil)
	c.Check(dname, Equals, filepath.Join(tmpDir, "pkg.name"))
}

// checks that the a second helper run of an already-running helper
// (for an app) goes to the backlog
func (s *poolSuite) TestSecondRunSameAppToBacklog(c *C) {
	ch := s.pool.Start()
	defer s.pool.Stop()

	app1 := clickhelp.MustParseAppId("com.example.test_test-app-1")
	input1 := &HelperInput{
		App:            app1,
		NotificationId: "foo1",
		Payload:        []byte(`"hello1"`),
	}
	app2 := clickhelp.MustParseAppId("com.example.test_test-app-1")
	input2 := &HelperInput{
		App:            app2,
		NotificationId: "foo2",
		Payload:        []byte(`"hello2"`),
	}

	c.Assert(app1.Base(), Equals, app2.Base())

	s.pool.Run("fake", input1)
	s.pool.Run("fake", input2)

	s.waitForArgs(c, "Launch")
	go s.fakeLauncher.done("0")
	takeNext(ch, c)

	// this is where we check that:
	c.Check(s.log.Captured(), Matches, `(?ms).* helper input backlog has grown to 1 entries.$`)
}

// checks that the an Nth helper run goes to the backlog
func (s *poolSuite) TestRunNthAppToBacklog(c *C) {
	s.pool.(*kindHelperPool).maxNum = 2
	ch := s.pool.Start()
	defer s.pool.Stop()

	app1 := clickhelp.MustParseAppId("com.example.test_test-app-1")
	input1 := &HelperInput{
		App:            app1,
		NotificationId: "foo1",
		Payload:        []byte(`"hello1"`),
	}
	app2 := clickhelp.MustParseAppId("com.example.test_test-app-2")
	input2 := &HelperInput{
		App:            app2,
		NotificationId: "foo2",
		Payload:        []byte(`"hello2"`),
	}
	app3 := clickhelp.MustParseAppId("com.example.test_test-app-3")
	input3 := &HelperInput{
		App:            app3,
		NotificationId: "foo3",
		Payload:        []byte(`"hello3"`),
	}

	s.pool.Run("fake", input1)
	s.waitForArgs(c, "Launch")

	s.pool.Run("fake", input2)
	s.log.ResetCapture()
	s.waitForArgs(c, "Launch")

	s.pool.Run("fake", input3)

	go s.fakeLauncher.done("0")
	s.waitForArgs(c, "Launch")

	res := takeNext(ch, c)
	c.Assert(res, NotNil)
	c.Assert(res.Input, NotNil)
	c.Assert(res.Input.App, NotNil)
	c.Assert(res.Input.App.Original(), Equals, "com.example.test_test-app-1")
	go s.fakeLauncher.done("1")
	go s.fakeLauncher.done("2")
	takeNext(ch, c)
	takeNext(ch, c)

	// this is the crux: we're checking that the third Run() went to the backlog.
	c.Check(s.log.Captured(), Matches,
		`(?ms).* helper input backlog has grown to 1 entries\.$.*shrunk to 0 entries\.$`)
}

func (s *poolSuite) TestRunBacklogFailedContinuesDiffApp(c *C) {
	s.pool.(*kindHelperPool).maxNum = 1
	ch := s.pool.Start()
	defer s.pool.Stop()

	app1 := clickhelp.MustParseAppId("com.example.test_test-app-1")
	input1 := &HelperInput{
		App:            app1,
		NotificationId: "foo1",
		Payload:        []byte(`"hello1"`),
	}
	app2 := clickhelp.MustParseAppId("com.example.test_test-app-2")
	input2 := &HelperInput{
		App:            app2,
		NotificationId: "foo2",
		Payload:        []byte(`"hello2"`),
	}
	app3 := clickhelp.MustParseAppId("com.example.test_test-app-3")
	input3 := &HelperInput{
		App:            app3,
		NotificationId: "foo3",
		Payload:        []byte(`"hello3"`),
	}
	app4 := clickhelp.MustParseAppId("com.example.test_test-app-4")
	input4 := &HelperInput{
		App:            app4,
		NotificationId: "foo4",
		Payload:        []byte(`"hello4"`),
	}

	s.pool.Run("fake", input1)
	s.waitForArgs(c, "Launch")
	s.pool.Run("NOT-THERE", input2) // this will fail
	s.pool.Run("fake", input3)
	s.pool.Run("fake", input4)

	go s.fakeLauncher.done("0")
	// Everything up to here was just set-up.
	//
	// What we're checking for is that, if a helper launch fails, the
	// next one in the backlog is picked up.
	c.Assert(takeNext(ch, c).Input.App, Equals, app1)
	c.Assert(takeNext(ch, c).Input.App, Equals, app2)
	go s.fakeLauncher.done("2")
	s.waitForArgs(c, "Launch")
	c.Check(s.log.Captured(), Matches,
		`(?ms).* helper input backlog has grown to 3 entries\.$.*shrunk to 1 entries\.$`)
}

func (s *poolSuite) TestBigBacklogShrinks(c *C) {
	oldBufSz := InputBufferSize
	InputBufferSize = 0
	defer func() { InputBufferSize = oldBufSz }()
	s.pool.(*kindHelperPool).maxNum = 1
	ch := s.pool.Start()
	defer s.pool.Stop()

	app := clickhelp.MustParseAppId("com.example.test_test-app")
	s.pool.Run("fake", &HelperInput{App: app, NotificationId: "0", Payload: []byte(`""`)})
	s.pool.Run("fake", &HelperInput{App: app, NotificationId: "1", Payload: []byte(`""`)})
	s.pool.Run("fake", &HelperInput{App: app, NotificationId: "2", Payload: []byte(`""`)})
	s.waitForArgs(c, "Launch")
	go s.fakeLauncher.done("0")
	takeNext(ch, c)
	// so now there's one done, one "running", and one more waiting.
	// kicking it forward one more notch before checking the logs:
	s.waitForArgs(c, "Launch")
	go s.fakeLauncher.done("1")
	takeNext(ch, c)
	// (two done, one "running")
	c.Check(s.log.Captured(), Matches, `(?ms).* shrunk to 1 entries\.$`)
	// and the backlog shrinker shrunk the backlog
	c.Check(s.log.Captured(), Matches, `(?ms).*copying backlog to avoid wasting too much space .*`)
}

func (s *poolSuite) TestBacklogShrinkerNilToNil(c *C) {
	pool := s.pool.(*kindHelperPool)
	c.Check(pool.shrinkBacklog(nil, 0), IsNil)
}

func (s *poolSuite) TestBacklogShrinkerEmptyToNil(c *C) {
	pool := s.pool.(*kindHelperPool)
	empty := []*HelperInput{nil, nil, nil}
	c.Check(pool.shrinkBacklog(empty, 0), IsNil)
}

func (s *poolSuite) TestBacklogShrinkerFullUntouched(c *C) {
	pool := s.pool.(*kindHelperPool)
	input := &HelperInput{}
	full := []*HelperInput{input, input, input}
	c.Check(pool.shrinkBacklog(full, 3), DeepEquals, full)
}

func (s *poolSuite) TestBacklogShrinkerSparseShrunk(c *C) {
	pool := s.pool.(*kindHelperPool)
	input := &HelperInput{}
	sparse := []*HelperInput{nil, input, nil, input, nil}
	full := []*HelperInput{input, input}
	c.Check(pool.shrinkBacklog(sparse, 2), DeepEquals, full)
}
