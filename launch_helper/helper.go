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

// launch_helper wraps ubuntu_app_launch to enable using application
// helpers. The useful part is HelperRunner
package launch_helper

/*
#cgo pkg-config: ubuntu-app-launch-2
#include <stdlib.h>
#include <ubuntu-app-launch.h>
#include <glib.h>

void stop_observer(const gchar * appid, const gchar * instanceid, const gchar * helpertype, gpointer user_data);
*/
import "C"

import (
	"time"
	"unsafe"

	"launchpad.net/ubuntu-push/logger"
)

type ReturnValue int

const (
	// HelperStopped means the helper was stopped forcefully
	HelperStopped ReturnValue = iota
	// HelperFinished means the helper ended normally
	HelperFinished
	// HelperFailed means the helper failed to start
	HelperFailed
	// StopFailed means tried to stop the helper but failed
	StopFailed
)

const timeLimit = 500 * time.Millisecond

// These are needed for testing because C functions can't be passed
// around as values
func _start_helper(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) C.gboolean {
	return C.ubuntu_app_launch_start_helper(helper_type, appid, uris)
}

var startHelper = _start_helper

func _stop_helper(helper_type *C.gchar, appid *C.gchar) C.gboolean {
	return C.ubuntu_app_launch_stop_helper(helper_type, appid)
}

var stopHelper = _stop_helper

// this channel is global because it needs to be accessed from goObserver which needs
// to be global to be exported
var finished = make(chan bool)

//export goObserver
func goObserver() {
	finished <- true
}

// Convert two strings into a proper NULL-terminated char**
func twoStringsForC(f1 string, f2 string) []*C.char {
	// 3 because we need a NULL terminator
	ptr := make([]*C.char, 3)
	ptr[0] = C.CString(f1)
	ptr[1] = C.CString(f2)
	return ptr
}

// run is a wrapper for ubuntu_app_launc_start_helper
func run(helper_type string, app_id string, fname1 string, fname2 string) bool {
	_helper_type := (*C.gchar)(C.CString(helper_type))
	defer C.free(unsafe.Pointer(_helper_type))
	_app_id := (*C.gchar)(C.CString(app_id))
	defer C.free(unsafe.Pointer(_app_id))
	c_fnames := twoStringsForC(fname1, fname2)
	defer C.free(unsafe.Pointer(c_fnames[0]))
	defer C.free(unsafe.Pointer(c_fnames[1]))
	success := startHelper(_helper_type, _app_id, (**C.gchar)(unsafe.Pointer(&c_fnames[0])))
	return (C.int)(success) != 0
}

// stop is a wrapper for ubuntu_app_launch_stop_helper
func stop(helper_type string, app_id string) bool {
	_helper_type := (*C.gchar)(C.CString(helper_type))
	defer C.free(unsafe.Pointer(_helper_type))
	_app_id := (*C.gchar)(C.CString(app_id))
	defer C.free(unsafe.Pointer(_app_id))
	success := stopHelper(_helper_type, _app_id)
	return (C.int)(success) != 0
}

// RunnerResult represent the result of running a helper
type RunnerResult struct {
	Status ReturnValue
	Helper []string
}

// New Creates a HelperRunner
//
// log is a logger to use.
func New(log logger.Logger, helper_type string) HelperRunner {
	input := make(chan []string)
	output := make(chan RunnerResult)
	return HelperRunner{
		log,
		input,
		output,
		helper_type,
	}

}

// Start launches the helper processes received in the helpers channel and
// puts results in the results channel.
// Should be called as a goroutine.
func (hr *HelperRunner) Start() {
	helper_type := (*C.gchar)(C.CString(hr.helper_type))
	defer C.free(unsafe.Pointer(helper_type))
	// Create an observer to be notified when helpers stop
	C.ubuntu_app_launch_observer_add_helper_stop(
		(C.UbuntuAppLaunchHelperObserver)(C.stop_observer),
		helper_type,
		nil,
	)
	for helper := range hr.Helpers {
		result := hr.Run(helper)
		hr.Results <- RunnerResult{result, helper}
	}
}

// HelperRunner is the struct used to launch helpers.
//
// Helpers is the input channel and gets (helper_type, appid, file1, file2)
//
// Results is the output channel, returns a RunnerResult struct.
//
// In that struct, helper is what was used as input and status is one of the ReturnValue constants defined in this package.
type HelperRunner struct {
	log         logger.Logger
	Helpers     chan []string
	Results     chan RunnerResult
	helper_type string
}

// Run starts a helper via ubuntu_app_launch_start_helper, and either
// waits for it to finish or stops it if more than timeLimit
// has passed.
//
// The helper argument is helper_type, appid, uri1, uri2
//
// The return value is a ReturnValue const defined in this package.
//
// You probably don't want to run this directly, but instead
// use Start
func (hr *HelperRunner) Run(helper []string) ReturnValue {
	timeout := make(chan bool)
	// Always start with a clean finished channel to avoid races
	finished = make(chan bool)
	hr.log.Debugf("Starting helper: %s %s %s %s", helper[0], helper[1], helper[2], helper[3])
	success := run(helper[0], helper[1], helper[2], helper[3])
	if success {
		go func() {
			time.Sleep(timeLimit)
			timeout <- true
		}()
		select {
		case <-timeout:
			hr.log.Debugf("Timeout reached, stopping")
			if stop(helper[0], helper[1]) {
				return HelperStopped
			} else {
				return StopFailed
			}
		case <-finished:
			hr.log.Debugf("Finished before timeout, doing nothing")
			return HelperFinished
		}
	} else {
		hr.log.Debugf("Failed to start helper")
		return HelperFailed
	}
}
