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

// helper_launcher wraps ubuntu_app_launch to enable using application
// helpers. The useful part is HelperRunner
package helper_launcher

/*
#cgo pkg-config: ubuntu-app-launch-2
#include <stdlib.h>
#include <ubuntu-app-launch.h>
#include <glib.h>

void stop_observer(const gchar * appid, const gchar * instanceid, const gchar * helpertype, gpointer user_data);
*/
import "C"
import "unsafe"
import "time"
import "launchpad.net/ubuntu-push/logger"

const (
	_timelimit     = 500 * time.Millisecond
	HelperStopped  = 1
	HelperFinished = 2
	HelperFailed   = 3
	StopFailed     = 4
)

// These are needed for testing because C functions can't be passed
// around as values
func _start_helper(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) C.gboolean {
	return C.ubuntu_app_launch_start_helper(helper_type, appid, uris)
}

var StartHelper = _start_helper

func _stop_helper(helper_type *C.gchar, appid *C.gchar) C.gboolean {
	return C.ubuntu_app_launch_stop_helper(helper_type, appid)
}

var StopHelper = _stop_helper

// this channel is global because it needs to be accessed from go_observer which needs
// to be global to be exported
var finished = make(chan bool)

//export go_observer
func go_observer() {
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

// Wrapper for ubuntu_app_launc_start_helper
func run(helper_type string, app_id string, fname1 string, fname2 string) bool {
	_helper_type := (*C.gchar)(C.CString(helper_type))
	defer C.free(unsafe.Pointer(_helper_type))
	_app_id := (*C.gchar)(C.CString(app_id))
	defer C.free(unsafe.Pointer(_app_id))
	c_fnames := twoStringsForC(fname1, fname2)
	defer C.free(unsafe.Pointer(c_fnames[0]))
	defer C.free(unsafe.Pointer(c_fnames[1]))
	success := StartHelper(_helper_type, _app_id, (**C.gchar)(unsafe.Pointer(&c_fnames[0])))
	return (C.int)(success) != 0
}

// Wrapper for ubuntu_app_launc_stop_helper
func stop(helper_type string, app_id string) bool {
	_helper_type := (*C.gchar)(C.CString(helper_type))
	defer C.free(unsafe.Pointer(_helper_type))
	_app_id := (*C.gchar)(C.CString(app_id))
	defer C.free(unsafe.Pointer(_app_id))
	success := StopHelper(_helper_type, _app_id)
	return (C.int)(success) != 0
}

// Struct for result of running a helper
type RunnerResult struct {
	status int
	helper []string
}


// helpers is the input channel and gets (helper_type, appid, file1, file2)
// results is the output channel, returns a RunnerResult struct.
// in that struct, helper is what was used as input and status is one of:
// HelperStopped: the helper was stopped forcefully
// HelperFinished: the helper ended normally
// HelperFailed: the helper failed to start
// StopFailed: tried to stop the helper but failed
// helper_type is the type of helpers this runner will launch.
type HelperRunner struct {
	log logger.Logger
	helpers chan []string
	results chan RunnerResult
	helper_type string
}

// Launc this in a goroutine to make the helper process requests and return results
func (hr *HelperRunner) run (){
	helper_type := (*C.gchar)(C.CString(hr.helper_type))
	defer C.free(unsafe.Pointer(helper_type))
	// Create an observer to be notified when helpers stop
	C.ubuntu_app_launch_observer_add_helper_stop(
		(C.UbuntuAppLaunchHelperObserver)(C.stop_observer),
		helper_type,
		nil,
	)
	for helper := range hr.helpers {
		result := hr.RunHelper(helper)
		hr.results <- RunnerResult{result, helper}
	}
}

// Creates a HelperRunner, returns the helper
// log is a logger to use.
func NewHelperRunner(log logger.Logger, helper_type string) HelperRunner{
	input := make(chan []string)
	output := make(chan RunnerResult)
	return HelperRunner {
		log,
		input,
		output,
		helper_type,
	}

}

// Starts a helper via ubuntu_app_launch_start_helper, and either
// wait for it to finish or stop it if more than _timilimit
// has passed.
//
// The helper argument is helper_type, appid, uri1, uri2
//
// The return value is one of:
// HelperStopped: the helper was stopped forcefully
// HelperFinished: the helper ended normally
// HelperFailed: the helper failed to start
// StopFailed: tried to stop the helper but failed
func (hr *HelperRunner) RunHelper(helper []string) int {
	timeout := make(chan bool)
	// Always start with a clean finished channel to avoid races
	finished = make(chan bool)
	hr.log.Debugf("Starting helper: %s %s %s %s", helper[0], helper[1], helper[2], helper[3])
	success := run(helper[0], helper[1], helper[2], helper[3])
	if success {
		go func() {
			time.Sleep(_timelimit)
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
