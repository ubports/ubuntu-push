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

package launch_helper

/*
#cgo pkg-config: ubuntu-app-launch-2
#include <ubuntu-app-launch.h>
#include <glib.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <glib.h>

extern void goObserver(const gchar *);
void stop_observer (const gchar * appid, const gchar * instanceid, const gchar * helpertype, gpointer user_data) {
    printf("%s | %s | %s \n", appid, instanceid, helpertype);
    goObserver(instanceid);
}

*/
import "C"

import (
	"fmt"
	"time"
	"unsafe"
)

// Utility functions to avoid typing the same casts too many times
func gchar(s string) *C.gchar {
	return (*C.gchar)(C.CString(s))
}

func free(s *C.gchar) {
	C.free(unsafe.Pointer(s))
}

// These functions are used by tests, they can't be defined in the test file
// because cgo is not allowed there

func fakeStartLongLivedHelper(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) *C.gchar {
	go func() {
		time.Sleep(timeLimit * 3)
		iid := gchar("hello")
		defer free(iid)
		goObserver(iid)
	}()
	return gchar("hello")
}

func fakeStartShortLivedHelper(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) *C.gchar {
	go func() {
		iid := gchar("hi")
		defer free(iid)
		goObserver(iid)
	}()
	return gchar("hi")
}

func fakeStartFailure(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) *C.gchar {
	return nil
}

func fakeStartCheckCasting(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) *C.gchar {

	if "foo1" != C.GoString((*C.char)(helper_type)) {
		fmt.Printf("helper_type is not properly casted")
		return gchar("hi")
	}

	if "bar1" != C.GoString((*C.char)(appid)) {
		fmt.Printf("appid is not properly casted")
		return gchar("hi")
	}

	var uri string
	q := uintptr(unsafe.Pointer(uris))
	p := (**C.char)(unsafe.Pointer(q))
	uri = C.GoString(*p)
	if uri != "bat1" {
		fmt.Printf("uri1 is not properly casted")
		return gchar("hi")
	}
	q += unsafe.Sizeof(q)
	p = (**C.char)(unsafe.Pointer(q))
	uri = C.GoString(*p)
	if uri != "baz1" {
		fmt.Printf("uri2 is not properly casted")
		return gchar("hi")
	}
	q += unsafe.Sizeof(q)
	p = (**C.char)(unsafe.Pointer(q))
	if *p != nil {
		fmt.Printf("uris is not NULL terminated")
		return gchar("hi")
	}

	return nil
}

func fakeStopCheckCasting(helper_type *C.gchar, app_id *C.gchar, instance_id *C.gchar) C.gboolean {

	if "foo1" != C.GoString((*C.char)(helper_type)) {
		fmt.Printf("helper_type is not properly casted")
		return C.TRUE
	}

	if "bar1" != C.GoString((*C.char)(app_id)) {
		fmt.Printf("app_id is not properly casted")
		return C.TRUE
	}

	if "hello" != C.GoString((*C.char)(instance_id)) {
		fmt.Printf("instance_id is not properly casted")
		return C.TRUE
	}

	finishedCh <- ""

	return C.FALSE
}

func fakeStop(helper_type *C.gchar, app_id *C.gchar, instance_id *C.gchar) C.gboolean {
	finishedCh <- ""

	return C.TRUE
}
