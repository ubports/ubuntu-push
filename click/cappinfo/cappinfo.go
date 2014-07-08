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

// Package cappinfo wraps C functions to access app information

package cappinfo

/*
#cgo pkg-config: gio-unix-2.0
#include <glib.h>
#include <gio/gdesktopappinfo.h>

gchar* app_icon_filename_from_desktop_id (gchar* desktop_id) {
    gchar* filename = NULL;
    GAppInfo* app_info = (GAppInfo*)g_desktop_app_info_new (desktop_id);
    if (app_info != NULL) {
        GIcon* icon = g_app_info_get_icon (app_info);
        if (icon != NULL) {
            filename = g_icon_to_string (icon);
            // g_app_info_get_icon has "transfer none"
        }
        g_object_unref (app_info);
    }
   g_free (desktop_id);
   return filename;
}
*/
import "C"

func AppIconFromDesktopId(desktopId string) string {
	name := C.app_icon_filename_from_desktop_id((*C.gchar)(C.CString(desktopId)))
	defer C.g_free((C.gpointer)(name))
	return C.GoString((*C.char)(name))
}
