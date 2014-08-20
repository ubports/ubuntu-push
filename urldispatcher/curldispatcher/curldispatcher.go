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

void dispatch_url(const gchar* url) {
    url_dispatch_send(url, NULL, NULL);
}

gchar** test_url(const gchar** urls) {
    char** result = url_dispatch_url_appid(urls);
    return result;
}
*/
import "C"
import "unsafe"

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
	packages := make([]string, len(urls))
	ptrSz := unsafe.Sizeof(unsafe.Pointer(nil))
	for p := uintptr(unsafe.Pointer(results)) + ptrSz; getCharPtr(p) != nil; p += ptrSz {
		pkg := C.GoString(getCharPtr(p))
		if pkg != "" { // ignore empty results
			packages = append(packages, pkg)
		}
	}
	return packages
}

func DispatchURL(url string, appPackage string) error {
	c_url := gchar(url)
	defer gfree(c_url)
	C.dispatch_url(c_url)
	return nil
}
