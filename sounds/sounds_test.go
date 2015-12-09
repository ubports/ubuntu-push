/*
 Copyright 2014 Canonical Ltd.

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

package sounds

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"testing"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/click"
	clickhelp "launchpad.net/ubuntu-push/click/testing"
	"launchpad.net/ubuntu-push/launch_helper"
	helpers "launchpad.net/ubuntu-push/testing"
)

func TestSounds(t *testing.T) { TestingT(t) }

type soundsSuite struct {
	log *helpers.TestLogger
	app *click.AppId
	acc *mockAccounts
}

var _ = Suite(&soundsSuite{})

type mockAccounts struct {
	vib bool
	sil bool
	snd string
	err error
}

func (m *mockAccounts) Start() error             { return m.err }
func (m *mockAccounts) Cancel() error            { return m.err }
func (m *mockAccounts) SilentMode() bool         { return m.sil }
func (m *mockAccounts) Vibrate() bool            { return m.vib }
func (m *mockAccounts) MessageSoundFile() string { return m.snd }
func (m *mockAccounts) String() string           { return "<mockAccounts>" }

func (ss *soundsSuite) SetUpTest(c *C) {
	ss.log = helpers.NewTestLogger(c, "debug")
	ss.app = clickhelp.MustParseAppId("com.example.test_test_0")
	ss.acc = &mockAccounts{true, false, "", nil}
}

func (ss *soundsSuite) TestNew(c *C) {
	s := New(ss.log, ss.acc, "foo")
	c.Check(s.log, Equals, ss.log)
	c.Check(s.player, Equals, "paplay")
	c.Check(s.fallback, Equals, "foo")
	c.Check(s.acc, Equals, ss.acc)
}

func (ss *soundsSuite) TestPresent(c *C) {
	s := &sound{
		player: "echo", log: ss.log, acc: ss.acc,
		dataFind: func(s string) (string, error) { return s, nil },
	}

	c.Check(s.Present(ss.app, "",
		&launch_helper.Notification{RawSound: json.RawMessage(`"hello"`)}), Equals, true)
	c.Check(ss.log.Captured(), Matches, `(?sm).* playing sound com.example.test/hello using echo`)
}

func (ss *soundsSuite) TestPresentSimple(c *C) {
	s := &sound{
		player: "echo", log: ss.log, acc: ss.acc,
		dataFind: func(s string) (string, error) { return s, nil },
		fallback: "fallback",
	}

	c.Check(s.Present(ss.app, "",
		&launch_helper.Notification{RawSound: json.RawMessage(`true`)}), Equals, true)
	c.Check(ss.log.Captured(), Matches, `(?sm).* playing sound com.example.test/fallback using echo`)
	ss.acc.snd = "from-prefs"
	ss.log.ResetCapture()
	c.Check(s.Present(ss.app, "",
		&launch_helper.Notification{RawSound: json.RawMessage(`true`)}), Equals, true)
	c.Check(ss.log.Captured(), Matches, `(?sm).* playing sound com.example.test/from-prefs using echo`)
}

func (ss *soundsSuite) TestPresentFails(c *C) {
	s := &sound{
		player:   "/",
		log:      ss.log,
		acc:      ss.acc,
		dataFind: func(string) (string, error) { return "", errors.New("nope") },
		dataDirs: func() []string { return []string{""} },
	}

	// nil notification
	c.Check(func() { s.Present(ss.app, "", nil) }, Panics, `please check notification is not nil before calling present`)
	// no Sound
	c.Check(s.Present(ss.app, "", &launch_helper.Notification{}), Equals, false)
	// bad player
	c.Check(s.Present(ss.app, "", &launch_helper.Notification{RawSound: json.RawMessage(`"hello"`)}), Equals, false)
	s.player = "echo"
	// no file found
	c.Check(s.Present(ss.app, "", &launch_helper.Notification{RawSound: json.RawMessage(`"hello"`)}), Equals, false)

	// and now, just to prove it would've worked,

	d := c.MkDir()
	f, err := os.Create(path.Join(d, "hello"))
	c.Assert(err, IsNil)
	f.Close()
	s.dataDirs = func() []string { return []string{"", d} }
	c.Check(s.Present(ss.app, "", &launch_helper.Notification{RawSound: json.RawMessage(`"hello"`)}), Equals, true)
}

func (ss *soundsSuite) TestBadPathFails(c *C) {
	s := &sound{
		player:   "/",
		log:      ss.log,
		acc:      ss.acc,
		dataFind: func(string) (string, error) { return "", errors.New("nope") },
		dataDirs: func() []string { return []string{""} },
	}

	sound, err := s.cleanPath("../../foo")
	c.Check(err, NotNil)
	c.Check(sound, Equals, "")
}

func (ss *soundsSuite) TestGoodPathSucceeds(c *C) {
	s := &sound{
		player:   "/",
		log:      ss.log,
		acc:      ss.acc,
		dataFind: func(string) (string, error) { return "", errors.New("nope") },
		dataDirs: func() []string { return []string{""} },
	}

	sound, err := s.cleanPath("foo/../bar")
	c.Check(err, IsNil)
	c.Check(sound, Equals, "bar")
}

func (ss *soundsSuite) TestSkipIfSilentMode(c *C) {
	s := &sound{
		player:   "echo",
		log:      ss.log,
		acc:      ss.acc,
		fallback: "fallback",
		dataFind: func(s string) (string, error) { return s, nil },
	}

	c.Check(s.Present(ss.app, "",
		&launch_helper.Notification{RawSound: json.RawMessage(`true`)}), Equals, true)
	ss.acc.sil = true
	c.Check(s.Present(ss.app, "",
		&launch_helper.Notification{RawSound: json.RawMessage(`true`)}), Equals, false)
}
