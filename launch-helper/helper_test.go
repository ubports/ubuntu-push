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
