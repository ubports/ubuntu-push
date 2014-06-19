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

package helper_launcher

/*
#cgo pkg-config: ubuntu-app-launch-2
#include <ubuntu-app-launch.h>
#include <glib.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <glib.h>

extern void goObserver();
void stop_observer (const gchar * appid, const gchar * instanceid, const gchar * helpertype, gpointer user_data) {
    printf("%s | %s | %s \n", appid, instanceid, helpertype);
    goObserver();
}

*/
import "C"

import (
	"fmt"
	"time"
	"unsafe"
)

// Utility functions to avoid typing the same casts too many times
func char(s string) *C.char {
	return (*C.char)(C.CString(s))
}

func free(s *C.char) {
	C.free(unsafe.Pointer(s))
}

// These functions are used by tests, they can't be defined in the test file
// because cgo is not allowed there

func fakeStartLongLivedHelper(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) C.gboolean {
	go func() {
		time.Sleep(timeLimit * 3)
		goObserver()
	}()
	return (C.gboolean)(1)
}

func fakeStartShortLivedHelper(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) C.gboolean {
	go func() {
		goObserver()
	}()
	return (C.gboolean)(1)
}

func fakeStartFailure(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) C.gboolean {
	return (C.gboolean)(0)
}

func fakeStartCheckCasting(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) C.gboolean {

	if "foo1" != C.GoString((*C.char)(helper_type)) {
		fmt.Printf("helper_type is not properly casted")
		return (C.gboolean)(1)
	}

	if "bar1" != C.GoString((*C.char)(appid)) {
		fmt.Printf("appid is not properly casted")
		return (C.gboolean)(1)
	}

	var uri string
	q := uintptr(unsafe.Pointer(uris))
	p := (**C.char)(unsafe.Pointer(q))
	uri = C.GoString(*p)
	if uri != "bat1" {
		fmt.Printf("uri1 is not properly casted")
		return (C.gboolean)(1)
	}
	q += unsafe.Sizeof(q)
	p = (**C.char)(unsafe.Pointer(q))
	uri = C.GoString(*p)
	if uri != "baz1" {
		fmt.Printf("uri2 is not properly casted")
		return (C.gboolean)(1)
	}
	q += unsafe.Sizeof(q)
	p = (**C.char)(unsafe.Pointer(q))
	if *p != nil {
		fmt.Printf("uris is not NULL terminated")
		return (C.gboolean)(1)
	}

	return (C.gboolean)(0)
}

func fakeStopCheckCasting(helper_type *C.gchar, appid *C.gchar) C.gboolean {

	if "foo1" != C.GoString((*C.char)(helper_type)) {
		fmt.Printf("helper_type is not properly casted")
		return (C.gboolean)(1)
	}

	if "bar1" != C.GoString((*C.char)(appid)) {
		fmt.Printf("appid is not properly casted")
		return (C.gboolean)(1)
	}
	return (C.gboolean)(0)
}

func fakeStop(helper_type *C.gchar, appid *C.gchar) C.gboolean {
	return (C.gboolean)(1)
}
