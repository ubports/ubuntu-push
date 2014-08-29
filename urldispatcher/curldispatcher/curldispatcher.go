/*
 Copyright 2014 Canonical Ltd.

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

// package cmessaging wraps libmessaging-menu
package curldispatcher

/*
#cgo pkg-config: url-dispatcher-1

#include <liburl-dispatcher-1/url-dispatcher.h>
#include <glib.h>

void dispatch_url(const gchar* url, gpointer user_data);

gchar** test_url(const gchar** urls);
*/
import "C"
import "unsafe"
import "fmt"

func gchar(s string) *C.gchar {
	return (*C.gchar)(C.CString(s))
}

func gfree(s *C.gchar) {
	C.g_free((C.gpointer)(s))
}

func getCharPtr(p uintptr) *C.char {
	return *((**C.char)(unsafe.Pointer(p)))
}

func TestURL(urls []string) []string {
	c_urls := make([]*C.gchar, len(urls)+1)
	for i, url := range urls {
		c_urls[i] = gchar(url)
		defer gfree(c_urls[i])
	}
	results := C.test_url((**C.gchar)(unsafe.Pointer(&c_urls[0])))
	// if there result is nil, just return empty []string
	if results == nil {
		return nil
	}
	packages := make([]string, len(urls))
	ptrSz := unsafe.Sizeof(unsafe.Pointer(nil))
	i := 0
	for p := uintptr(unsafe.Pointer(results)); getCharPtr(p) != nil; p += ptrSz {
		pkg := C.GoString(getCharPtr(p))
		packages[i] = pkg
		i += 1
	}
	return packages
}

type DispatchPayload struct {
	doneCh chan bool
}

func DispatchURL(url string, appPackage string) error {
	c_url := gchar(url)
	defer gfree(c_url)
	c_app_package := gchar(appPackage)
	defer gfree(c_app_package)
	doneCh := make(chan bool)
	payload := DispatchPayload{doneCh: doneCh}
	C.dispatch_url(c_url, (C.gpointer)(&payload))
	success := <-doneCh
	if !success {
		return fmt.Errorf("Failed to DispatchURL: %s for %s", url, appPackage)
	}
	return nil
}

//export handleDispatchURLResult
func handleDispatchURLResult(c_action *C.char, c_success C.gboolean, obj unsafe.Pointer) {
	payload := (*DispatchPayload)(obj)
	var success bool
	if c_success == C.TRUE {
		success = true
	}
	payload.doneCh <- success
}
