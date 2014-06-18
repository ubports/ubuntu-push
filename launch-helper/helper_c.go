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
import "fmt"
import "time"

// These functions are used by tests, they can't be defined in the test file
// because cgo is not allowed there

func FakeStartLongLivedHelper(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) C.gboolean {
	go func() {
		fmt.Printf("timeout1\n")
		time.Sleep(_timelimit * 1000)
		fmt.Printf("timeout2 calling observer\n")
		go_observer()
	}()
	return (C.gboolean)(1)
}
func FakeStartShortLivedHelper(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) C.gboolean {
	go func() {
		go_observer()
	}()
	return (C.gboolean)(1)
}

func FakeStop(helper_type *C.gchar, appid *C.gchar) C.gboolean {
	fmt.Printf("Using the fake stop\n")
	return (C.gboolean)(1)
}

