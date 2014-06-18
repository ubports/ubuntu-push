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

// These functions are used by tests, they can't be defined in the test file
// because cgo is not allowed there
func FakeStart(helper_type *C.gchar, appid *C.gchar, uris **C.gchar) C.gboolean {
	fmt.Printf("Using the fake start\n")
	return (C.gboolean)(1)
}
func FakeStop(helper_type *C.gchar, appid *C.gchar) C.gboolean {
	fmt.Printf("Using the fake start\n")
	return (C.gboolean)(1)
}

