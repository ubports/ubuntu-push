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

package logger

import (
	"bytes"
	"fmt"
	. "launchpad.net/gocheck"
	"os"
	"runtime"
	"testing"
)

func TestLogger(t *testing.T) { TestingT(t) }

type loggerSuite struct{}

var _ = Suite(&loggerSuite{})

func (s *loggerSuite) TestErrorf(c *C) {
	buf := &bytes.Buffer{}
	logger := NewSimpleLogger(buf, "error")
	logger.Errorf("%v %d", "error", 1)
	c.Check(buf.String(), Matches, ".* ERROR error 1\n")
}

func (s *loggerSuite) TestFatalf(c *C) {
	defer func() {
		osExit = os.Exit
	}()
	var exitCode int
	osExit = func(code int) {
		exitCode = code
	}
	buf := &bytes.Buffer{}
	logger := NewSimpleLogger(buf, "error")
	logger.Fatalf("%v %v", "error", "fatal")
	c.Check(buf.String(), Matches, ".* ERROR error fatal\n")
	c.Check(exitCode, Equals, 1)
}

func (s *loggerSuite) TestInfof(c *C) {
	buf := &bytes.Buffer{}
	logger := NewSimpleLogger(buf, "info")
	logger.Infof("%v %d", "info", 1)
	c.Check(buf.String(), Matches, ".* INFO info 1\n")
}

func (s *loggerSuite) TestDebugf(c *C) {
	buf := &bytes.Buffer{}
	logger := NewSimpleLogger(buf, "debug")
	logger.Debugf("%v %d", "debug", 1)
	c.Check(buf.String(), Matches, `.* DEBUG debug 1\n`)
}

func (s *loggerSuite) TestFormat(c *C) {
	buf := &bytes.Buffer{}
	logger := NewSimpleLogger(buf, "error")
	logger.Errorf("%v %d", "error", 2)
	c.Check(buf.String(), Matches, `.* .*\.\d+ ERROR error 2\n`)
}

func (s *loggerSuite) TestLevel(c *C) {
	buf := &bytes.Buffer{}
	logger := NewSimpleLogger(buf, "error")
	logger.Errorf("%s%d", "e", 3)
	logger.Infof("%s%d", "i", 3)
	logger.Debugf("%s%d", "d", 3)
	c.Check(buf.String(), Matches, `.* ERROR e3\n`)

	buf.Reset()
	logger = NewSimpleLogger(buf, "info")
	logger.Errorf("%s%d", "e", 4)
	logger.Debugf("%s%d", "d", 4)
	logger.Infof("%s%d", "i", 4)
	c.Check(buf.String(), Matches, `.* ERROR e4\n.* INFO i4\n`)

	buf.Reset()
	logger = NewSimpleLogger(buf, "debug")
	logger.Errorf("%s%d", "e", 5)
	logger.Debugf("%s%d", "d", 5)
	logger.Infof("%s%d", "i", 5)
	c.Check(buf.String(), Matches, `.* ERROR e5\n.* DEBUG d5\n.* INFO i5\n`)
}

func panicAndRecover(logger Logger, n int, doPanic bool, line *int, ok *bool) {
	defer func() {
		if err := recover(); err != nil {
			logger.PanicStackf("%v %d", err, n)
		}
	}()
	_, _, *line, *ok = runtime.Caller(0)
	if doPanic {
		panic("Troubles") // @ line + 2
	}
}

func (s *loggerSuite) TestPanicStackfPanicScenario(c *C) {
	buf := &bytes.Buffer{}
	logger := NewSimpleLogger(buf, "error")
	var line int
	var ok bool
	panicAndRecover(logger, 6, true, &line, &ok)
	c.Assert(ok, Equals, true)
	c.Check(buf.String(), Matches, fmt.Sprintf("(?s).* ERROR\\(PANIC\\) Troubles 6:.*panicAndRecover.*logger_test.go:%d.*", line+2))
}

func (s *loggerSuite) TestPanicStackfNoPanicScenario(c *C) {
	buf := &bytes.Buffer{}
	logger := NewSimpleLogger(buf, "error")
	var line int
	var ok bool
	panicAndRecover(logger, 6, false, &line, &ok)
	c.Check(buf.String(), Equals, "")
}
