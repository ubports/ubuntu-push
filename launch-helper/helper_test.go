package main

import "testing"

var runnerTests = []struct {
	expected int // expected result
	msg string // description of failure
	starter func(*_Ctype_gchar, *_Ctype_gchar, **_Ctype_gchar) _Ctype_gboolean // starter fake
	stopper func(*_Ctype_gchar, *_Ctype_gchar) _Ctype_gboolean // stopper fake
} {
	{helper_stopped, "Long running helper is not stopped", fakeStartLongLivedHelper, fakeStop},
	{helper_finished, "Short running helper doesn't finish", fakeStartShortLivedHelper, fakeStop},
	{helper_failed, "Filure to start helper doesn't fail", fakeStartFailure, fakeStop},
	{helper_failed, "Error in argument casting", fakeStartCheckCasting, fakeStop},
}


func TestRunner(t *testing.T) {
	for _, tt := range runnerTests {
		StartHelper = tt.starter
		StopHelper = tt.stopper
		command :=[]string{"foo1", "bar1", "bat1", "baz1"}
		if runner(command) != tt.expected {
			t.Fatalf(tt.msg)
		}
	}
}
