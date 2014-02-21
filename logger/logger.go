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

// Package logger defines a simple logger API with level of logging control.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
)

// Logger is a simple logger interface with logging at levels.
type Logger interface {
	// Re-expose base Output for logging events.
	Output(calldept int, s string) error
	// Errorf logs an error.
	Errorf(format string, v ...interface{})
	// Fatalf logs an error and exits the program with os.Exit(1).
	Fatalf(format string, v ...interface{})
	// PanicStackf logs an error message and a stacktrace, for use
	// in panic recovery.
	PanicStackf(format string, v ...interface{})
	// Infof logs an info message.
	Infof(format string, v ...interface{})
	// Debugf logs a debug message.
	Debugf(format string, v ...interface{})
}

type simpleLogger struct {
	outputFunc func(calldepth int, s string) error
	nlevel     int
}

const (
	lError = iota
	lInfo
	lDebug
)

var levelToNLevel = map[string]int{
	"error": lError,
	"info":  lInfo,
	"debug": lDebug,
}

// MinimalLogger is the  minimal interface required to build a simple logger.
type MinimalLogger interface {
	Output(calldepth int, s string) error
}

// NewSimpleLoggerFromMinimalLogger creates a logger logging only up
// to the given level. level can be in order: "error", "info",
// "debug". It takes a value just implementing stlib Logger.Output().
func NewSimpleLoggerFromMinimalLogger(minLog MinimalLogger, level string) Logger {
	nlevel := levelToNLevel[level]
	return &simpleLogger{
		minLog.Output,
		nlevel,
	}
}

// NewSimpleLogger creates a logger logging only up to the given
// level. level can be in order: "error", "info", "debug". It takes an
// io.Writer.
func NewSimpleLogger(w io.Writer, level string) Logger {
	return NewSimpleLoggerFromMinimalLogger(
		log.New(w, "", log.Ldate|log.Ltime|log.Lmicroseconds),
		level,
	)
}

func (lg *simpleLogger) Output(calldepth int, s string) error {
	return lg.outputFunc(calldepth+2, s)
}

func (lg *simpleLogger) Errorf(format string, v ...interface{}) {
	lg.outputFunc(2, fmt.Sprintf("ERROR "+format, v...))
}

var osExit = os.Exit // for testing

func (lg *simpleLogger) Fatalf(format string, v ...interface{}) {
	lg.outputFunc(2, fmt.Sprintf("ERROR "+format, v...))
	osExit(1)
}

func (lg *simpleLogger) PanicStackf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	stack := make([]byte, 8*1024) // Stack writes less but doesn't fail
	stackWritten := runtime.Stack(stack, false)
	stack = stack[:stackWritten]
	lg.outputFunc(2, fmt.Sprintf("ERROR(PANIC) %s:\n%s", msg, stack))
}

func (lg *simpleLogger) Infof(format string, v ...interface{}) {
	if lg.nlevel >= lInfo {
		lg.outputFunc(2, fmt.Sprintf("INFO "+format, v...))
	}
}

func (lg *simpleLogger) Debugf(format string, v ...interface{}) {
	if lg.nlevel >= lDebug {
		lg.outputFunc(2, fmt.Sprintf("DEBUG "+format, v...))
	}
}
