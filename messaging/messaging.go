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

// Package messaging wraps the messaging menu indicator, allowing for persistent
// notifications to the user.
package messaging

/*
#cgo pkg-config: messaging-menu
#include <glib.h>
#include <messaging-menu/messaging-menu-app.h>
*/
import "C"

const (
  Available = C.MESSAGING_MENU_STATUS_AVAILABLE
  Away = C.MESSAGING_MENU_STATUS_AWAY
  Busy = C.MESSAGING_MENU_STATUS_BUSY
  Invisible = C.MESSAGING_MENU_STATUS_INVISIBLE
  Offline = C.MESSAGING_MENU_STATUS_OFFLINE
)

func MessagingMenuApp_new(desktop_id string) *C.struct_MessagingMenuApp {
    return C.messaging_menu_app_new((*C.gchar)(C.CString(desktop_id)))
}

func MessagingMenuApp_register(app *C.struct_MessagingMenuApp) {
    C.messaging_menu_app_register(app)
}

func MessagingMenuApp_unregister(app *C.struct_MessagingMenuApp) {
    C.messaging_menu_app_unregister(app)
}

func MessagingMenuApp_set_status(app *C.struct_MessagingMenuApp, status C.MessagingMenuStatus) {
    C.messaging_menu_app_set_status(app, status)
}

// FIXME: need a way to create a GIcon
func MessagingMenuApp_insert_source(app *C.struct_MessagingMenuApp, position int, id string, icon *C.GIcon, label string) {
    C.messaging_menu_app_insert_source(app, (C.gint)(C.int(position)), (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)))
}

func MessagingMenuApp_append_source(app *C.struct_MessagingMenuApp, id string, icon *C.GIcon, label string) {
    C.messaging_menu_app_append_source(app, (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)))
}

func MessagingMenuApp_insert_source_with_count(app *C.struct_MessagingMenuApp, position int, id string, icon *C.GIcon, label string, count int) {
    C.messaging_menu_app_insert_source_with_count(app, (C.gint)(C.int(position)), (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (C.guint)(C.uint(count)))
}

func MessagingMenuApp_append_source_with_count(app *C.struct_MessagingMenuApp, position int, id string, icon *C.GIcon, label string, count int) {
    C.messaging_menu_app_append_source_with_count(app, (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (C.guint)(C.uint(count)))
}
