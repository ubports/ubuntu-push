package main

/*
#cgo pkg-config: ubuntu-app-launch-2
#include <ubuntu-app-launch.h>
#include <glib.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <glib.h>

extern void go_observer();
void stop_observer (const gchar * appid, const gchar * instanceid, const gchar * helpertype, gpointer user_data) {
    printf("%s | %s | %s \n", appid, instanceid, helpertype);
    go_observer();
}

*/
import "C"
import "time"
import "fmt"
import "unsafe"

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
		time.Sleep(_timelimit * 3)
		go_observer()
	}()
	return (C.gboolean)(1)
}

func fakeStartShortLivedHelper(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) C.gboolean {
	go func() {
		go_observer()
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



