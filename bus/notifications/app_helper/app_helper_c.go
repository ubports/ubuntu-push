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

// Package app_helper wraps C functions to access app information
package app_helper

/*
#cgo pkg-config: gio-unix-2.0
#cgo pkg-config: gio-2.0
#include <stdlib.h>
#include <glib.h>
#include <gio/gdesktopappinfo.h>
*/
import "C"
import "unsafe"

func AppNameFromId(appId string) string {
	_id := C.CString(appId)
	_app_info := C.g_desktop_app_info_new(_id)
	_app_name := C.g_desktop_app_info_get_generic_name(_app_info)
	name := C.GoString(_app_name)
	C.free(unsafe.Pointer(_id))
	C.free(unsafe.Pointer(_app_name))
	C.free(unsafe.Pointer(_app_info))
	return name
}
