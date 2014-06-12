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
#include <messaging-menu/messaging-menu-message.h>
*/
import "C"
import "unsafe"


const (
  Available = C.MESSAGING_MENU_STATUS_AVAILABLE
  Away = C.MESSAGING_MENU_STATUS_AWAY
  Busy = C.MESSAGING_MENU_STATUS_BUSY
  Invisible = C.MESSAGING_MENU_STATUS_INVISIBLE
  Offline = C.MESSAGING_MENU_STATUS_OFFLINE
)

type MessagingMenuApp struct {
    instance *C.struct_MessagingMenuApp
}

type MessagingMenuMessage struct {
    instance *C.struct_MessagingMenuMessage
}

func NewApp(desktop_id string) MessagingMenuApp {
    return MessagingMenuApp{C.messaging_menu_app_new((*C.gchar)(C.CString(desktop_id)))}
}

func (app *MessagingMenuApp) Register() {
    C.messaging_menu_app_register(app.instance)
}

func (app *MessagingMenuApp) Unregister() {
    C.messaging_menu_app_unregister(app.instance)
}

func (app *MessagingMenuApp) SetStatus(status C.MessagingMenuStatus) {
    C.messaging_menu_app_set_status(app.instance, status)
}

// FIXME: need a way to create a GIcon, use nil in the meantime
func (app *MessagingMenuApp) InsertSource(position int, id string, icon *C.GIcon, label string) {
    C.messaging_menu_app_insert_source(app.instance, (C.gint)(C.int(position)), (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)))
}

func (app *MessagingMenuApp) AppendSource(id string, icon *C.GIcon, label string) {
    C.messaging_menu_app_append_source(app.instance, (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)))
}

func (app *MessagingMenuApp) InsertSourceWithCount(position int, id string, icon *C.GIcon, label string, count int) {
    C.messaging_menu_app_insert_source_with_count(app.instance, (C.gint)(C.int(position)), (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (C.guint)(C.uint(count)))
}

func (app *MessagingMenuApp) AppendSourceWithCount(id string, icon *C.GIcon, label string, count int) {
    C.messaging_menu_app_append_source_with_count(app.instance, (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (C.guint)(C.uint(count)))
}

func (app *MessagingMenuApp) InsertSourceWithTime(position int, id string, icon *C.GIcon, label string, time int) {
    C.messaging_menu_app_insert_source_with_time(app.instance, (C.gint)(C.int(position)), (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (C.gint64)(C.int(time)))
}

func (app *MessagingMenuApp) AppendSourceWithTime(id string, icon *C.GIcon, label string, time int) {
    C.messaging_menu_app_append_source_with_time(app.instance, (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (C.gint64)(C.int(time)))
}

func (app *MessagingMenuApp) InsertSourceWithString(position int, id string, icon *C.GIcon, label string, str string) {
    C.messaging_menu_app_insert_source_with_string(app.instance, (C.gint)(C.int(position)), (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (*C.gchar)(C.CString(str)))
}

func (app *MessagingMenuApp) AppendSourceWithString(id string, icon *C.GIcon, label string, str string) {
    C.messaging_menu_app_append_source_with_string(app.instance, (*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(label)), (*C.gchar)(C.CString(str)))
}

func (app *MessagingMenuApp) RemoveSource(id string) {
    C.messaging_menu_app_remove_source(app.instance, (*C.gchar)(C.CString(id)))
}

func (app *MessagingMenuApp) HasSource(id string) bool {
    var has_it = (C.int)(C.messaging_menu_app_has_source(app.instance, (*C.gchar)(C.CString(id))))
    return has_it != 0
}

func (app *MessagingMenuApp) SetSourceLabel(id string, label string) {
    C.messaging_menu_app_set_source_label(app.instance, (*C.gchar)(C.CString(id)), (*C.gchar)(C.CString(label)))
}

func (app *MessagingMenuApp) SetSourceIcon(id string, icon *C.GIcon) {
    C.messaging_menu_app_set_source_icon(app.instance, (*C.gchar)(C.CString(id)), icon)
}

func (app *MessagingMenuApp) SetSourceCount(id string, count int) {
    C.messaging_menu_app_set_source_count(app.instance, (*C.gchar)(C.CString(id)), (C.guint)(C.uint(count)))
}

func (app *MessagingMenuApp) SetSourceTime(id string, time int) {
    C.messaging_menu_app_set_source_time(app.instance, (*C.gchar)(C.CString(id)), (C.gint64)(C.uint(time)))
}

func (app *MessagingMenuApp) SetSourceString(id string, str string) {
    C.messaging_menu_app_set_source_string(app.instance, (*C.gchar)(C.CString(id)), (*C.gchar)(C.CString(str)))
}

func (app *MessagingMenuApp) DrawAttention(id string) {
    C.messaging_menu_app_draw_attention(app.instance, (*C.gchar)(C.CString(id)))
}

func (app *MessagingMenuApp) RemoveAttention(id string) {
    C.messaging_menu_app_remove_attention(app.instance, (*C.gchar)(C.CString(id)))
}

func (app *MessagingMenuApp) AppendMessage(msg MessagingMenuMessage, id string, notify bool) {
    if notify {  // FIXME: how to convert from bool to int?
        C.messaging_menu_app_append_message(app.instance, msg.instance, (*C.gchar)(C.CString(id)), (C.gboolean)(C.int(1)))
    } else {
        C.messaging_menu_app_append_message(app.instance, msg.instance, (*C.gchar)(C.CString(id)), (C.gboolean)(C.int(0)))
    }
}

func (app *MessagingMenuApp) GetMessage(id string) MessagingMenuMessage {
    return MessagingMenuMessage{C.messaging_menu_app_get_message(app.instance, (*C.gchar)(C.CString(id)))}
}

func (app *MessagingMenuApp) RemoveMessage(msg MessagingMenuMessage) {
    C.messaging_menu_app_remove_message(app.instance, msg.instance)
}

func (app *MessagingMenuApp) RemoveMessageById(id string) {
    C.messaging_menu_app_remove_message_by_id(app.instance, (*C.gchar)(C.CString(id)))
}

func NewMessage(id string, icon *C.GIcon, title string, subtitle string, body string, time int) MessagingMenuMessage {
    return MessagingMenuMessage{C.messaging_menu_message_new((*C.gchar)(C.CString(id)), icon, (*C.gchar)(C.CString(title)),
                                      (*C.gchar)(C.CString(subtitle)), (*C.gchar)(C.CString(body)), (C.gint64)(C.int(time)))}
}

func (msg *MessagingMenuMessage) GetId() string {
    return C.GoString   ((*C.char)(C.messaging_menu_message_get_id(msg.instance)))
}

func (msg *MessagingMenuMessage) GetIcon() *C.GIcon {
    return C.messaging_menu_message_get_icon(msg.instance)
}

func (msg *MessagingMenuMessage) GetTitle() string {
    return C.GoString((*C.char)(C.messaging_menu_message_get_title(msg.instance)))
}

func (msg *MessagingMenuMessage) GetSubtitle() string {
    return C.GoString((*C.char)(C.messaging_menu_message_get_subtitle(msg.instance)))
}

func (msg *MessagingMenuMessage) GetBody() string {
    return C.GoString((*C.char)(C.messaging_menu_message_get_body(msg.instance)))
}

func (msg *MessagingMenuMessage) GetTime() int {
    return int((C.int)(C.messaging_menu_message_get_time(msg.instance)))
}

func (msg *MessagingMenuMessage) GetDrawsAttention() bool {
    return int((C.int)(C.messaging_menu_message_get_draws_attention(msg.instance))) != 0
}

func (msg *MessagingMenuMessage) SetDrawsAttention(draws_attention bool) {
    if draws_attention {  // FIXME: how to convert from bool to int?
        C.messaging_menu_message_set_draws_attention(msg.instance, (C.gboolean)(C.int(1)))
    } else {
        C.messaging_menu_message_set_draws_attention(msg.instance, (C.gboolean)(C.int(0)))
    }
}


// Not wrapping this one... GVariantType + GVariant? How would I wrap that?
/*
void
messaging_menu_message_add_action (MessagingMenuMessage *msg,
                                   const gchar *id,
                                   const gchar *label,
                                   const GVariantType *parameter_type,
                                   GVariant *parameter_hint);*/

type SourceActivatedCallback func()

func SignalConnectObject(instance MessagingMenuApp, detailed_signal string, callback unsafe.Pointer, gobject C.gpointer) {
    C.g_signal_connect_object(
        (C.gpointer)(instance.instance),
        (*C.gchar)(C.CString(detailed_signal)),
        (C.GCallback)(callback),
        gobject,
        C.G_CONNECT_AFTER)
}

// YUCK

func EnterMainLoop() {
    var loop = C.g_main_loop_new(nil, 0)
    go C.g_main_loop_run(loop)
}
