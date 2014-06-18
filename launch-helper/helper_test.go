package helper_launcher

import "testing"

var runnerTests = []struct {
	expected int                                                                // expected result
	msg      string                                                             // description of failure
	starter  func(*_Ctype_gchar, *_Ctype_gchar, **_Ctype_gchar) _Ctype_gboolean // starter fake
	stopper  func(*_Ctype_gchar, *_Ctype_gchar) _Ctype_gboolean                 // stopper fake
}{
	{HelperStopped, "Long running helper is not stopped", fakeStartLongLivedHelper, fakeStop},
	{HelperFinished, "Short running helper doesn't finish", fakeStartShortLivedHelper, fakeStop},
	{HelperFailed, "Filure to start helper doesn't fail", fakeStartFailure, fakeStop},
	{HelperFailed, "Error in start argument casting", fakeStartCheckCasting, fakeStop},
	{StopFailed, "Error in stop argument casting", fakeStartLongLivedHelper, fakeStopCheckCasting},
}

func TestRunner(t *testing.T) {
	for _, tt := range runnerTests {
		StartHelper = tt.starter
		StopHelper = tt.stopper
		command := []string{"foo1", "bar1", "bat1", "baz1"}
		if runHelper(command) != tt.expected {
			t.Fatalf(tt.msg)
		}
	}
}
