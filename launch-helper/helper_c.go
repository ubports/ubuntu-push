package main

/*
#cgo pkg-config: ubuntu-app-launch-2
#include <stdio.h>
#include <glib.h>

extern void go_observer();
void stop_observer (const gchar * appid, const gchar * instanceid, const gchar * helpertype, gpointer user_data) {
    printf("%s | %s | %s \n", appid, instanceid, helpertype);
    go_observer();
}

*/
import "C"
import "time"

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

func fakeStop(helper_type *C.gchar, appid *C.gchar) C.gboolean {
	return (C.gboolean)(1)
}

