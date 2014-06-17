package main

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

var finished = make(chan bool)

const (
	_timelimit = 500
)

//export go_observer
func go_observer() {
	finished <- true
}

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
	success := C.ubuntu_app_launch_start_helper(_helper_type, _app_id, (**C.gchar)(unsafe.Pointer(&c_fnames[0])))
	return (C.int)(success) != 0
}

func stop(helper_type string, app_id string) bool {
	_helper_type := (*C.gchar)(C.CString(helper_type))
	defer C.free(unsafe.Pointer(_helper_type))
	_app_id := (*C.gchar)(C.CString(app_id))
	defer C.free(unsafe.Pointer(_app_id))
	success := C.ubuntu_app_launch_stop_helper(_helper_type, _app_id)
	return (C.int)(success) != 0
}

func runner(commands chan []string) {
	for {
		command := <-commands
		timeout := make(chan bool)

		helper_type := (*C.gchar)(C.CString("foobar"))
		defer C.free(unsafe.Pointer(helper_type))
		// Create an observer to be notified when helpers stop

		C.ubuntu_app_launch_observer_add_helper_stop(
			(C.UbuntuAppLaunchHelperObserver)(C.stop_observer),
			helper_type,
			nil,
		)
        success := run(command[0], command[1], command[2], command[3])
		if success {
			go func() {
				time.Sleep(_timelimit * time.Millisecond)
				timeout <- true
			}()
			select {
				case <-timeout:
					stop(command[0], command[1])
				case <-finished:
					fmt.Printf("Finished before timeout, doing nothing\n")
			}
		} else {
			fmt.Printf("Failed to start helper\n")
		}
	}
}

func main() {
	commands := make(chan []string)
	commandList := [][]string{
		[]string{"foo1", "bar1", "bat1", "baz1"},
		[]string{"foo2", "bar2", "bat2", "baz2"},
	}

	go runner(commands)
	for _, command := range commandList {
		fmt.Printf("sending %s\n", command)
		commands <- command
	}
	time.Sleep(4 * 1e9)
}
