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
	"errors"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
	"unsafe"

	"launchpad.net/go-xdg/v0"

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
func _start_helper(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) *C.gchar {
	return C.ubuntu_app_launch_start_multiple_helper(helper_type, appid, uris)
}

var startHelper = _start_helper

func _stop_helper(helper_type *C.gchar, app_id *C.gchar, instance_id *C.gchar) C.gboolean {
	return C.ubuntu_app_launch_stop_multiple_helper(helper_type, app_id, instance_id)
}

var stopHelper = _stop_helper

// this channel is global because it needs to be accessed from goObserver which needs
// to be global to be exported
var finishedCh = make(chan bool, 1)

//export goObserver
func goObserver() {
	finishedCh <- true
}

// Convert two strings into a proper NULL-terminated gchar**
func twoStringsForC(f1 string, f2 string) []*C.gchar {
	// 3 because we need a NULL terminator
	ptr := make([]*C.gchar, 3)
	ptr[0] = gchar(f1)
	ptr[1] = gchar(f2)
	return ptr
}

// run is a wrapper for ubuntu_app_launc_start_multiple_helper
//
// XXX: also return an error
func run(helperType string, appId string, uri1 string, uri2 string) string {
	helper_type := gchar(helperType)
	defer free(helper_type)
	app_id := gchar(appId)
	defer free(app_id)
	c_uris := twoStringsForC(uri1, uri2)
	defer free(c_uris[0])
	defer free(c_uris[1])
	instance_id := startHelper(helper_type, app_id, (**C.gchar)(unsafe.Pointer(&c_uris[0])))
	if instance_id == nil {
		return ""
	}
	defer free(instance_id)
	return C.GoString((*C.char)(instance_id))
}

// stop is a wrapper for ubuntu_app_launch_stop_multiple_helper
func stop(helperType string, appId string, instanceId string) bool {
	helper_type := gchar(helperType)
	defer free(helper_type)
	app_id := gchar(appId)
	defer free(app_id)
	instance_id := gchar(instanceId)
	defer free(instance_id)
	success := stopHelper(helper_type, app_id, instance_id)
	return (C.int)(success) != 0
}

// HelperArgs represent the arguments for the helper
type HelperArgs struct {
	AppId   string
	Payload []byte
	Input   string
	Output  string
}

// RunnerResult represent the result of running a helper
type RunnerResult struct {
	Status ReturnValue
	Helper HelperArgs
	Data   []byte
	Error  error
}

// New Creates a HelperRunner
//
// log is a logger to use.
func New(log logger.Logger, helperType string) HelperRunner {
	input := make(chan HelperArgs)
	output := make(chan RunnerResult)
	return HelperRunner{
		log,
		input,
		output,
		helperType,
	}

}

// Start launches the helper processes received in the helpers channel and
// puts results in the results channel.
// Should be called as a goroutine.
func (hr *HelperRunner) Start() {
	helper_type := gchar(hr.helperType)
	defer free(helper_type)
	// Create an observer to be notified when helpers stop
	C.ubuntu_app_launch_observer_add_helper_stop(
		(C.UbuntuAppLaunchHelperObserver)(C.stop_observer),
		helper_type,
		nil,
	)
	for helper := range hr.Helpers {
		// create in/output files
		pkgName := strings.Split(helper.AppId, "_")[0]
		err := hr.createTempFiles(&helper, pkgName)
		if err != nil {
			hr.log.Errorf("failed to create temp files: %v", err)
			hr.Results <- RunnerResult{HelperFailed, helper, nil, err}
			continue
		}
		err = ioutil.WriteFile(helper.Input, helper.Payload, os.ModeTemporary)
		if err != nil {
			hr.log.Errorf("failed to write to input file: %v", err)
			os.Remove(helper.Input)
			os.Remove(helper.Output)
			hr.Results <- RunnerResult{HelperFailed, helper, nil, err}
			continue
		}
		result := hr.Run(helper.AppId, helper.Input, helper.Output)
		// read the output file and build the result
		if result != HelperFinished && result != HelperStopped && result != StopFailed {
			hr.log.Errorf("helper run failed with: %v, %v", helper, result)
			os.Remove(helper.Input)
			os.Remove(helper.Output)
			hr.Results <- RunnerResult{result, helper, nil, errors.New("Helper failed.")}
			continue
		}
		data, err := ioutil.ReadFile(helper.Output)
		os.Remove(helper.Input)
		os.Remove(helper.Output)
		if err != nil {
			hr.log.Errorf("failed to read output file: %v", err)
			hr.Results <- RunnerResult{HelperFailed, helper, nil, err}
		} else {
			hr.Results <- RunnerResult{result, helper, data, nil}
		}
	}
}

// HelperRunner is the struct used to launch helpers.
//
// Helpers is the input channel and gets (helperType, appid, file1, file2)
//
// Results is the output channel, returns a RunnerResult struct.
//
// In that struct, helper is what was used as input and status is one of the ReturnValue constants defined in this package.
type HelperRunner struct {
	log        logger.Logger
	Helpers    chan HelperArgs
	Results    chan RunnerResult
	helperType string
}

// Run starts a helper via ubuntu_app_launch_start_helper, and either
// waits for it to finish or stops it if more than timeLimit
// has passed.
//
// The return value is a ReturnValue const defined in this package.
//
// You probably don't want to run this directly, but instead
// use Start
func (hr *HelperRunner) Run(appId string, input string, output string) ReturnValue {
	hr.log.Debugf("starting helper: %s %s %s %s", hr.helperType, appId, input, output)
	instanceId := run(hr.helperType, appId, input, output)
	if instanceId == "" {
		hr.log.Debugf("failed to start helper")
		return HelperFailed
	}

	select {
	case <-time.After(timeLimit):
		hr.log.Debugf("timeout reached, stopping")
		if stop(hr.helperType, appId, instanceId) {
			// wait for the stop to come in
			<-finishedCh
			return HelperStopped
		} else {
			return StopFailed
		}
	case <-finishedCh:
		hr.log.Debugf("finished before timeout, doing nothing")
		return HelperFinished
	}
}

var xdgCacheHome = xdg.Cache.Home

func _getTempDir(pkgName string) (string, error) {
	tmpDir := path.Join(xdgCacheHome(), pkgName)
	err := os.MkdirAll(path.Dir(tmpDir), 0700)
	return tmpDir, err
}

var getTempDir = _getTempDir

func _getTempFilename(pkgName string) (string, error) {
	tmpDir, err := getTempDir(pkgName)
	if err != nil {
		return "", err
	}
	file, err := ioutil.TempFile(tmpDir, "push-helper")
	defer file.Close()
	if err != nil {
		return "", err
	}
	return file.Name(), nil
}

var getTempFilename = _getTempFilename

func (hr *HelperRunner) createTempFiles(helper *HelperArgs, pkgName string) error {
	var err error
	if helper.Input == "" {
		helper.Input, err = getTempFilename(pkgName)
		if err != nil {
			hr.log.Errorf("failed to create input file: %v", err)
			return err
		}
	}
	if helper.Output == "" {
		helper.Output, err = getTempFilename(pkgName)
		if err != nil {
			hr.log.Errorf("failed to create output file: %v", err)
			return err
		}
	}
	return nil
}
