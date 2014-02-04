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
	"launchpad.net/ubuntu-push/logger"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type captureHelper struct {
	outputFunc func(int, string) error
	lock       sync.Mutex
	logEvents  []string
	written    *chan bool
}

func (h *captureHelper) Output(calldepth int, s string) error {
	err := h.outputFunc(calldepth+2, s)
	if err == nil {
		h.lock.Lock()
		defer h.lock.Unlock()
		if *h.written != nil {
			*h.written <- true
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

// TestLogger implements logger.Logger using gocheck.C and supporting
// capturing log strings.
type TestLogger struct {
	logger.Logger
	helper  *captureHelper
	Written chan bool
}

// NewTestLogger can be used in tests instead of NewSimpleLogger(FromMinimalLogger).
func NewTestLogger(minLog interface {
	Output(int, string) error
}, level string) *TestLogger {
	h := &captureHelper{outputFunc: minLog.Output}
	log := &TestLogger{
		Logger: logger.NewSimpleLoggerFromMinimalLogger(h, level),
		helper: h,
	}
	h.written = &log.Written
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

// SourceRelative produces a path relative to the source code, makes
// sense only for tests when the code is available on disk.
func SourceRelative(relativePath string) string {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		panic("failed to get source filename using Caller()")
	}
	return filepath.Join(filepath.Dir(file), relativePath)
}
