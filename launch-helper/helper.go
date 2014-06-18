package helper_launcher

/*
#cgo pkg-config: ubuntu-app-launch-2
#include <stdlib.h>
#include <ubuntu-app-launch.h>
#include <glib.h>

void stop_observer(const gchar * appid, const gchar * instanceid, const gchar * helpertype, gpointer user_data);
*/
import "C"
import "fmt"
import "unsafe"
import "time"

const (
	_timelimit = 500 * time.Millisecond
	helper_stopped = 1
	helper_finished = 2
	helper_failed = 3
	stop_failed = 4
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

func stop(helper_type string, app_id string) bool {
	_helper_type := (*C.gchar)(C.CString(helper_type))
	defer C.free(unsafe.Pointer(_helper_type))
	_app_id := (*C.gchar)(C.CString(app_id))
	defer C.free(unsafe.Pointer(_app_id))
	success := StopHelper(_helper_type, _app_id)
	return (C.int)(success) != 0
}

func runHelper(helper []string) int {
	timeout := make(chan bool)
	// Always start with a clean finished channel to avoid races
	finished = make(chan bool)
	success := run(helper[0], helper[1], helper[2], helper[3])
	if success {
		go func() {
			time.Sleep(_timelimit)
			timeout <- true
		}()
		select {
			case <-timeout:
				fmt.Printf("Timeout reached, stopping\n")
				if stop(helper[0], helper[1]) {
					return helper_stopped
				} else {
					return stop_failed
				}
			case <-finished:
				fmt.Printf("Finished before timeout, doing nothing\n")
				return helper_finished
		}
	} else {
		fmt.Printf("Failed to start helper\n")
		return helper_failed
	}
}

func HelperRunner(helpers chan []string) {
	// XXX obviously not foobar
	helper_type := (*C.gchar)(C.CString("foobar"))
	defer C.free(unsafe.Pointer(helper_type))
	// Create an observer to be notified when helpers stop
	C.ubuntu_app_launch_observer_add_helper_stop(
		(C.UbuntuAppLaunchHelperObserver)(C.stop_observer),
		helper_type,
		nil,
	)
	for helper := range helpers {
		runHelper(helper)
	}
}
