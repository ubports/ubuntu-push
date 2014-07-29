package helper_finder

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	. "launchpad.net/gocheck"
	helpers "launchpad.net/ubuntu-push/testing"

	"launchpad.net/ubuntu-push/click"
)

type helperSuite struct {
	oldHookPath        string
	symlinkPath        string
	oldHelpersDataPath string
	log                *helpers.TestLogger
}

func TestHelperFinder(t *testing.T) { TestingT(t) }

var _ = Suite(&helperSuite{})

func (s *helperSuite) SetUpTest(c *C) {
	s.oldHookPath = hookPath
	hookPath = c.MkDir()
	s.symlinkPath = c.MkDir()
	s.oldHelpersDataPath = helpersDataPath
	helpersDataPath = filepath.Join(c.MkDir(), "helpers_data.json")
	s.log = helpers.NewTestLogger(c, "debug")
}

func (s *helperSuite) createHookfile(name string, content string) error {
	symlink := filepath.Join(hookPath, name) + ".json"
	filename := filepath.Join(s.symlinkPath, name)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	_, err = f.WriteString(content)
	if err != nil {
		return err
	}
	err = os.Symlink(filename, symlink)
	if err != nil {
		return err
	}
	return nil
}

func (s *helperSuite) createHelpersDatafile(content string) error {
	f, err := os.Create(helpersDataPath)
	if err != nil {
		return err
	}
	_, err = f.WriteString(content)
	if err != nil {
		return err
	}
	return nil
}

func (s *helperSuite) TearDownTest(c *C) {
	hookPath = s.oldHookPath
	os.Remove(helpersDataPath)
	helpersDataPath = s.oldHelpersDataPath
	helpersDataMtime = time.Now().Add(-1 * time.Hour)
	helpersInfo = nil
}

func (s *helperSuite) TestHelperBasic(c *C) {
	c.Assert(s.createHelpersDatafile(`{"com.example.test": {"helper_id": "com.example.test_test-helper_1", "exec": "tsthlpr"}}`), IsNil)
	app, err := click.ParseAppId("com.example.test_test-app_1")
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "com.example.test_test-helper_1")
	c.Check(hex, Equals, "tsthlpr")
}

func (s *helperSuite) TestHelperFindsSpecific(c *C) {
	fileContent := `{"com.example.test_test-other-app": {"exec": "aaaaaaa", "helper_id": "com.example.test_aaaa-helper_1"},
    "com.example.test_test-app": {"exec": "tsthlpr", "helper_id": "com.example.test_test-helper_1"}}`
	c.Assert(s.createHelpersDatafile(fileContent), IsNil)

	app, err := click.ParseAppId("com.example.test_test-app_1")
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "com.example.test_test-helper_1")
	c.Check(hex, Equals, "tsthlpr")
}

func (s *helperSuite) TestHelperCanFail(c *C) {
	fileContent := `{"com.example.test_test-other-app": {"exec": "aaaaaaa", "helper_id": "com.example.test_aaaa-helper_1"}}`
	c.Assert(s.createHelpersDatafile(fileContent), IsNil)
	app, err := click.ParseAppId("com.example.test_test-app_1")
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "")
	c.Check(hex, Equals, "")
}

func (s *helperSuite) TestHelperFailInvalidJson(c *C) {
	fileContent := `{invalid json"com.example.test_test-other-app": {"exec": "aaaaaaa", "helper_id": "com.example.test_aaaa-helper_1"}}`
	c.Assert(s.createHelpersDatafile(fileContent), IsNil)
	app, err := click.ParseAppId("com.example.test_test-app_1")
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "")
	c.Check(hex, Equals, "")
}

func (s *helperSuite) TestHelperFailMissingExec(c *C) {
	fileContent := `{"com.example.test_test-app": {"helper_id": "com.example.test_aaaa-helper_1"}}`
	c.Assert(s.createHelpersDatafile(fileContent), IsNil)
	app, err := click.ParseAppId("com.example.test_test-app_1")
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "")
	c.Check(hex, Equals, "")
}

func (s *helperSuite) TestHelperlegacy(c *C) {
	appname := "ubuntu-system-settings"
	app, err := click.ParseAppId("_" + appname)
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "")
	c.Check(hex, Equals, "")
}

// Missing Cache file test

func (s *helperSuite) TestHelperMissingCacheFile(c *C) {
	c.Assert(s.createHookfile("com.example.test_test-helper_1", `{"exec": "tsthlpr"}`), IsNil)
	app, err := click.ParseAppId("com.example.test_test-app_1")
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "com.example.test_test-helper_1")
	c.Check(hex, Equals, filepath.Join(s.symlinkPath, "tsthlpr"))
	c.Check(s.log.Captured(), Matches, ".*Cache file not found, falling back to .json file lookup\n")
}

func (s *helperSuite) TestHelperFromHookBasic(c *C) {
	c.Assert(s.createHookfile("com.example.test_test-helper_1", `{"exec": "tsthlpr"}`), IsNil)
	app, err := click.ParseAppId("com.example.test_test-app_1")
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "com.example.test_test-helper_1")
	c.Check(hex, Equals, filepath.Join(s.symlinkPath, "tsthlpr"))
}

func (s *helperSuite) TestHelperFromHookFindsSpecific(c *C) {
	// Glob() sorts, so the first one will come first
	c.Assert(s.createHookfile("com.example.test_aaaa-helper_1", `{"exec": "aaaaaaa", "app_id": "com.example.test_test-other-app"}`), IsNil)
	c.Assert(s.createHookfile("com.example.test_test-helper_1", `{"exec": "tsthlpr", "app_id": "com.example.test_test-app"}`), IsNil)
	app, err := click.ParseAppId("com.example.test_test-app_1")
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "com.example.test_test-helper_1")
	c.Check(hex, Equals, filepath.Join(s.symlinkPath, "tsthlpr"))
}

func (s *helperSuite) TestHelperFromHookCanFail(c *C) {
	c.Assert(s.createHookfile("com.example.test_aaaa-helper_1", `{"exec": "aaaaaaa", "app_id": "com.example.test_test-other-app"}`), IsNil)
	app, err := click.ParseAppId("com.example.test_test-app_1")
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "")
	c.Check(hex, Equals, "")
}

func (s *helperSuite) TestHelperFromHookInvalidJson(c *C) {
	c.Assert(s.createHookfile("com.example.test_aaaa-helper_1", `invalid json {"exec": "aaaaaaa", "app_id": "com.example.test_test-other-app"}`), IsNil)
	app, err := click.ParseAppId("com.example.test_test-app_1")
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "")
	c.Check(hex, Equals, "")
}

func (s *helperSuite) TestHelperFromHooFailBrokenSymlink(c *C) {
	name := "com.example.test_aaaa-helper_1"
	c.Assert(s.createHookfile(name, `{"exec": "aaaaaaa", "app_id": "com.example.test_test-other-app"}`), IsNil)
	filename := filepath.Join(s.symlinkPath, name)
	os.Remove(filename)
	app, err := click.ParseAppId("com.example.test_test-app_1")
	c.Assert(err, IsNil)
	hid, hex := Helper(app, s.log)
	c.Check(hid, Equals, "")
	c.Check(hex, Equals, "")
}
