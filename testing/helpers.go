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

// Package testing contains helpers for testing.
package testing

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
)

type captureHelper struct {
	outputFunc func(int, string) error
	lock       sync.Mutex
	logEvents  []string
	logEventCb func(string)
}

func (h *captureHelper) Output(calldepth int, s string) error {
	err := h.outputFunc(calldepth+2, s)
	if err == nil {
		h.lock.Lock()
		defer h.lock.Unlock()
		if h.logEventCb != nil {
			h.logEventCb(s)
		}
		h.logEvents = append(h.logEvents, s+"\n")
	}
	return err
}

func (h *captureHelper) captured() string {
	h.lock.Lock()
	defer h.lock.Unlock()
	return strings.Join(h.logEvents, "")
}

func (h *captureHelper) reset() {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.logEvents = nil
}

func (h *captureHelper) setLogEventCb(cb func(string)) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.logEventCb = cb
}

// TestLogger implements logger.Logger using gocheck.C and supporting
// capturing log strings.
type TestLogger struct {
	logger.Logger
	helper *captureHelper
}

// NewTestLogger can be used in tests instead of
// NewSimpleLogger(FromMinimalLogger).
func NewTestLogger(minLog logger.MinimalLogger, level string) *TestLogger {
	h := &captureHelper{outputFunc: minLog.Output}
	log := &TestLogger{
		Logger: logger.NewSimpleLoggerFromMinimalLogger(h, level),
		helper: h,
	}
	return log
}

// Captured returns accumulated log events.
func (tlog *TestLogger) Captured() string {
	return tlog.helper.captured()
}

// Reset resets accumulated log events.
func (tlog *TestLogger) ResetCapture() {
	tlog.helper.reset()
}

// SetLogEventCb sets a callback invoked for log events.
func (tlog *TestLogger) SetLogEventCb(cb func(string)) {
	tlog.helper.setLogEventCb(cb)
}

// SourceRelative produces a path relative to the source code, makes
// sense only for tests when the code is available on disk.
func SourceRelative(relativePath string) string {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		panic("failed to get source filename using Caller()")
	}
	dir := filepath.Dir(file)

	root := os.Getenv("UBUNTU_PUSH_TEST_RESOURCES_ROOT")
	if root != "" {
		const sep = "launchpad.net/ubuntu-push/"

		idx := strings.LastIndex(dir, sep)
		if idx == -1 {
			panic(fmt.Errorf("Unable to find %s in %#v", sep, dir))
		}
		idx += len(sep)

		dir = filepath.Join(root, dir[idx:])
	}
	return filepath.Join(dir, relativePath)
}

// ScriptAbsPath gets the absolute path to a script in the scripts directory
// assuming we're in a subdirectory of the project
func ScriptAbsPath(script string) string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("unable to get working directory: %v", err))
	}

	if !path.IsAbs(cwd) {
		panic(fmt.Errorf("working directory not absolute? %v", cwd))
	}

	for cwd != "/" {
		filename := path.Join(cwd, "scripts", script)
		_, err := os.Stat(filename)
		if err == nil {
			return filename
		}
		if !os.IsNotExist(err) {
			panic(fmt.Errorf("unable to stat %v: %v", filename, err))
		}
		cwd = path.Dir(cwd)
	}
	panic(fmt.Errorf("unable to find script %v", script))
}

// Ns makes a []Notification from just payloads.
func Ns(payloads ...json.RawMessage) []protocol.Notification {
	res := make([]protocol.Notification, len(payloads))
	for i := 0; i < len(payloads); i++ {
		res[i].Payload = payloads[i]
	}
	return res
}

// ParseURL parses a URL conveniently.
func ParseURL(s string) *url.URL {
	purl, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return purl
}

func MustParseAppId(appId string) *click.AppId {
	app, err := click.ParseAppId(appId)
	if err != nil {
		panic(err)
	}
	return app
}
