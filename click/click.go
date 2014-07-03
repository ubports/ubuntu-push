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

// Package click wraps libclick to check if packages are installed.
package click

/*
#cgo pkg-config: click-0.4
#cgo LDFLAGS: -lglib-2.0 -lgobject-2.0

#include <click-0.4/click.h>
*/
import "C"

import (
	"fmt"
	"runtime"
	"strings"
)

type ClickUser struct {
	cuser *C.ClickUser
}

func gchar(s string) *C.gchar {
	return (*C.gchar)(C.CString(s))
}

func gfree(s *C.gchar) {
	C.g_free((C.gpointer)(s))
}

// User makes a new ClickUser object for the current user.
func User() (*ClickUser, error) {
	var gerr *C.GError
	cuser := C.click_user_new_for_user(nil, nil, &gerr)
	defer C.g_clear_error(&gerr)
	if gerr != nil {
		return nil, fmt.Errorf("faild to make ClickUser: %s", C.GoString((*C.char)(gerr.message)))
	}
	res := &ClickUser{cuser}
	runtime.SetFinalizer(res, func(cu *ClickUser) {
		C.g_object_unref((C.gpointer)(cu.cuser))
	})
	return res, nil
}

// HasPackage checks if the appId is installed for user.
func (cu *ClickUser) HasPackage(appId string) bool {
	comps := strings.Split(appId, "_")
	if len(comps) < 2 {
		return false
	}
	pkgname := gchar(comps[0])
	defer gfree(pkgname)
	switch len(comps) {
	case 3: // with version
		var gerr *C.GError
		defer C.g_clear_error(&gerr)
		ver := C.click_user_get_version(cu.cuser, pkgname, &gerr)
		if gerr != nil {
			return false
		}
		defer gfree(ver)
		return C.GoString((*C.char)(ver)) == comps[2]
	case 2:
		return C.click_user_has_package_name(cu.cuser, pkgname) == C.TRUE
	default:
		return false
	}
}
