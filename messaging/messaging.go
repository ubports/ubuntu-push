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

func MessagingMenuApp_append_source_with_count(app *C.struct_MessagingMenuApp, id string, icon *C.GIcon, label string, count int) {
    C.messaging_menu_app_append_source_with_count(app, (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (C.guint)(C.uint(count)))
}

func MessagingMenuApp_insert_source_with_time(app *C.struct_MessagingMenuApp, position int, id string, icon *C.GIcon, label string, time int) {
    C.messaging_menu_app_insert_source_with_time(app, (C.gint)(C.int(position)), (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (C.gint64)(C.int(time)))
}

func MessagingMenuApp_append_source_with_time(app *C.struct_MessagingMenuApp, id string, icon *C.GIcon, label string, time int) {
    C.messaging_menu_app_append_source_with_time(app, (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (C.gint64)(C.int(time)))
}

func MessagingMenuApp_insert_source_with_string(app *C.struct_MessagingMenuApp, position int, id string, icon *C.GIcon, label string, str string) {
    C.messaging_menu_app_insert_source_with_string(app, (C.gint)(C.int(position)), (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (*C.gchar)(C.CString(str)))
}

func MessagingMenuApp_append_source_with_string(app *C.struct_MessagingMenuApp, id string, icon *C.GIcon, label string, str string) {
    C.messaging_menu_app_append_source_with_string(app, (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (*C.gchar)(C.CString(str)))
}

func MessagingMenuApp_remove_source(app *C.struct_MessagingMenuApp, id string) {
    C.messaging_menu_app_remove_source(app, (*C.gchar)(C.CString(id)))
}

func MessagingMenuApp_has_source(app *C.struct_MessagingMenuApp, id string) bool {
    var has_it = (C.int)(C.messaging_menu_app_has_source(app, (*C.gchar)(C.CString(id))))
    return has_it != 0
}

func MessagingMenuApp_set_source_label(app *C.struct_MessagingMenuApp, id string, label string) {
    C.messaging_menu_app_set_source_label(app, (*C.gchar)(C.CString(id)), (*C.gchar)(C.CString(label)))
}

func MessagingMenuApp_set_source_icon(app *C.struct_MessagingMenuApp, id string, icon *C.GIcon) {
    C.messaging_menu_app_set_source_icon(app, (*C.gchar)(C.CString(id)), icon)
}

func MessagingMenuApp_set_source_count(app *C.struct_MessagingMenuApp, id string, count int) {
    C.messaging_menu_app_set_source_count(app, (*C.gchar)(C.CString(id)), (C.guint)(C.uint(count)))
}

func MessagingMenuApp_set_source_time(app *C.struct_MessagingMenuApp, id string, time int) {
    C.messaging_menu_app_set_source_time(app, (*C.gchar)(C.CString(id)), (C.gint64)(C.uint(time)))
}

func MessagingMenuApp_set_source_string(app *C.struct_MessagingMenuApp, id string, str string) {
    C.messaging_menu_app_set_source_string(app, (*C.gchar)(C.CString(id)), (*C.gchar)(C.CString(str)))
}

func MessagingMenuApp_draw_attention(app *C.struct_MessagingMenuApp, id string) {
    C.messaging_menu_app_draw_attention(app, (*C.gchar)(C.CString(id)))
}

func MessagingMenuApp_remove_attention(app *C.struct_MessagingMenuApp, id string) {
    C.messaging_menu_app_remove_attention(app, (*C.gchar)(C.CString(id)))
}


// YUCK

func EnterMainLoop() {
    var loop = C.g_main_loop_new(nil, 0)
    C.g_main_loop_run(loop)
}
