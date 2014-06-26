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

func AppIconFromId(appId string) string {
	_id := C.CString(appId)
	defer C.free(unsafe.Pointer(_id))
	_app_info := C.g_desktop_app_info_new(_id)
 	defer C.g_app_info_delete(_app_info)
	_app_icon := C.g_app_info_get_icon(_app_info)
	defer C.g_object_unref((C.gpointer)(_app_icon))
	_icon_string := C.g_icon_to_string(_app_icon)
	defer C.free(unsafe.Pointer(_icon_string))
	name := C.GoString((*C.char)(_icon_string))
	return name
}
