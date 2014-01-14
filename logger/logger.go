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
	// Errorf logs an error.
	Errorf(format string, v ...interface{})
	// Fatalf logs an error and exists the program with os.Exit(1).
	Fatalf(format string, v ...interface{})
	// Recoverf recover from a possible panic and logs it.
	Recoverf(format string, v ...interface{})
	// Infof logs a info message.
	Infof(format string, v ...interface{})
	// Debugf logs a debug message.
	Debugf(format string, v ...interface{})
}

type simpleLogger struct {
	*log.Logger
	nlevel int
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

// NewSimpleLogger creates a logger logging only up to the given level.
// level can be in order: "error", "info", "debug".
func NewSimpleLogger(w io.Writer, level string) Logger {
	nlevel := levelToNLevel[level]
	return &simpleLogger{
		log.New(w, "", log.Ldate|log.Ltime|log.Lmicroseconds),
		nlevel,
	}
}

func (lg *simpleLogger) Errorf(format string, v ...interface{}) {
	lg.Printf("ERROR "+format, v...)
}

var osExit = os.Exit // for testing

func (lg *simpleLogger) Fatalf(format string, v ...interface{}) {
	lg.Printf("ERROR "+format, v...)
	osExit(1)
}

func (lg *simpleLogger) Recoverf(format string, v ...interface{}) {
	if err := recover(); err != nil {
		msg := fmt.Sprintf(format, v...)
		stack := make([]byte, 8*1024) // Stack writes less but doesn't fail
		stackWritten := runtime.Stack(stack, false)
		stack = stack[:stackWritten]
		lg.Printf("ERROR panic %v!! %s:\n%s", err, msg, stack)
	}
}

func (lg *simpleLogger) Infof(format string, v ...interface{}) {
	if lg.nlevel >= lInfo {
		lg.Printf("INFO "+format, v...)
	}
}

func (lg *simpleLogger) Debugf(format string, v ...interface{}) {
	if lg.nlevel >= lDebug {
		lg.Printf("DEBUG "+format, v...)
	}
}
