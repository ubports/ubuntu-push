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
#include <stdlib.h>
#include <glib.h>
#include <messaging-menu/messaging-menu-app.h>
#include <messaging-menu/messaging-menu-message.h>
*/
import "C"
import "unsafe"

const (
	Available = C.MESSAGING_MENU_STATUS_AVAILABLE
	Away      = C.MESSAGING_MENU_STATUS_AWAY
	Busy      = C.MESSAGING_MENU_STATUS_BUSY
	Invisible = C.MESSAGING_MENU_STATUS_INVISIBLE
	Offline   = C.MESSAGING_MENU_STATUS_OFFLINE
)

// MessagingMenuApp wraps an instance of libmessaging-menu's MessagingMenuApp and
// related functions.
//
// For individual functions, please refer to the documentation for the function of
// similar name at http://goo.gl/E0U2wu
type MessagingMenuApp struct {
	instance *C.struct_MessagingMenuApp
}

// MessagingMenuMenu wraps an instance of libmessaging-menu's MessagingMenuMessage and
// related functions.
//
// For individual functions, please refer to the documentation for the function of
// similar name at http://goo.gl/L62WTg
type MessagingMenuMessage struct {
	instance *C.struct_MessagingMenuMessage
}

// NewApp creates a MessagingMenuApp
func NewApp(desktop_id string) MessagingMenuApp {
	var _desktop_id = gchar(desktop_id)
	var app = MessagingMenuApp{C.messaging_menu_app_new(_desktop_id)}
	free(_desktop_id)
	return app
}

// NewMessage creates a MessagingMenuMessage
func NewMessage(id string, icon *C.GIcon, title string, subtitle string, body string, time int) MessagingMenuMessage {
	var _id = gchar(id)
	var _title = gchar(title)
	var _subtitle = gchar(subtitle)
	var _body = gchar(body)
	var msg = MessagingMenuMessage{C.messaging_menu_message_new(_id, icon, _title,
		_subtitle, _body, (C.gint64)(C.int(time)))}
	free(_id)
	free(_title)
	free(_subtitle)
	free(_body)
	return msg
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

// Utility functions to avoid typing the same casts too many times
func gchar(s string) *C.gchar {
	return (*C.gchar)(C.CString(s))
}

func free(s *C.gchar) {
	C.free(unsafe.Pointer(s))
}

// FIXME: need a way to create a GIcon, use nil in the meantime
func (app *MessagingMenuApp) InsertSource(position int, id string, icon *C.GIcon, label string) {
	var _id = gchar(id)
	var _label = gchar(label)
	C.messaging_menu_app_insert_source(app.instance, (C.gint)(C.int(position)), _id, icon, _label)
	free(_id)
	free(_label)
}

func (app *MessagingMenuApp) AppendSource(id string, icon *C.GIcon, label string) {
	var _id = gchar(id)
	var _label = gchar(label)
	C.messaging_menu_app_append_source(app.instance, _id, icon, _label)
	free(_id)
	free(_label)
}

func (app *MessagingMenuApp) InsertSourceWithCount(position int, id string, icon *C.GIcon, label string, count int) {
	var _id = gchar(id)
	var _label = gchar(label)
	C.messaging_menu_app_insert_source_with_count(app.instance, (C.gint)(C.int(position)), _id, icon, _label, (C.guint)(C.uint(count)))
	free(_id)
	free(_label)
}

func (app *MessagingMenuApp) AppendSourceWithCount(id string, icon *C.GIcon, label string, count int) {
	var _id = gchar(id)
	var _label = gchar(label)
	C.messaging_menu_app_append_source_with_count(app.instance, _id, icon, _label, (C.guint)(C.uint(count)))
	free(_id)
	free(_label)
}

func (app *MessagingMenuApp) InsertSourceWithTime(position int, id string, icon *C.GIcon, label string, time int) {
	var _id = gchar(id)
	var _label = gchar(label)
	C.messaging_menu_app_insert_source_with_time(app.instance, (C.gint)(C.int(position)), _id, icon, _label, (C.gint64)(C.int(time)))
	free(_id)
	free(_label)
}

func (app *MessagingMenuApp) AppendSourceWithTime(id string, icon *C.GIcon, label string, time int) {
	var _id = gchar(id)
	var _label = gchar(label)
	C.messaging_menu_app_append_source_with_time(app.instance, _id, icon, _label, (C.gint64)(C.int(time)))
	free(_id)
	free(_label)
}

func (app *MessagingMenuApp) InsertSourceWithString(position int, id string, icon *C.GIcon, label string, str string) {
	var _id = gchar(id)
	var _label = gchar(label)
	var _str = gchar(str)
	C.messaging_menu_app_insert_source_with_string(app.instance, (C.gint)(C.int(position)), _id, icon, _label, _str)
	free(_id)
	free(_label)
	free(_str)
}

func (app *MessagingMenuApp) AppendSourceWithString(id string, icon *C.GIcon, label string, str string) {
	var _id = gchar(id)
	var _label = gchar(label)
	var _str = gchar(str)
	C.messaging_menu_app_append_source_with_string(app.instance, _id, icon, _label, _str)
	free(_id)
	free(_label)
	free(_str)
}

func (app *MessagingMenuApp) RemoveSource(id string) {
	var _id = gchar(id)
	C.messaging_menu_app_remove_source(app.instance, _id)
	free(_id)
}

func (app *MessagingMenuApp) HasSource(id string) bool {
	var _id = gchar(id)
	var has_it = (C.int)(C.messaging_menu_app_has_source(app.instance, _id))
	free(_id)
	return has_it != 0
}

func (app *MessagingMenuApp) SetSourceLabel(id string, label string) {
	var _id = gchar(id)
	var _label = gchar(label)
	C.messaging_menu_app_set_source_label(app.instance, _id, _label)
	free(_id)
	free(_label)
}

func (app *MessagingMenuApp) SetSourceIcon(id string, icon *C.GIcon) {
	var _id = gchar(id)
	C.messaging_menu_app_set_source_icon(app.instance, _id, icon)
	free(_id)
}

func (app *MessagingMenuApp) SetSourceCount(id string, count int) {
	var _id = gchar(id)
	C.messaging_menu_app_set_source_count(app.instance, _id, (C.guint)(C.uint(count)))
	free(_id)
}

func (app *MessagingMenuApp) SetSourceTime(id string, time int) {
	var _id = gchar(id)
	C.messaging_menu_app_set_source_time(app.instance, _id, (C.gint64)(C.uint(time)))
	free(_id)
}

func (app *MessagingMenuApp) SetSourceString(id string, str string) {
	var _id = gchar(id)
	var _str = gchar(str)
	C.messaging_menu_app_set_source_string(app.instance, _id, _str)
	free(_id)
	free(_str)
}

func (app *MessagingMenuApp) DrawAttention(id string) {
	var _id = gchar(id)
	C.messaging_menu_app_draw_attention(app.instance, _id)
	free(_id)
}

func (app *MessagingMenuApp) RemoveAttention(id string) {
	var _id = gchar(id)
	C.messaging_menu_app_remove_attention(app.instance, _id)
	free(_id)
}

func (app *MessagingMenuApp) AppendMessage(msg MessagingMenuMessage, id string, notify bool) {
	var _id = gchar(id)
	if notify { // FIXME: how to convert from bool to int?
		C.messaging_menu_app_append_message(app.instance, msg.instance, _id, (C.gboolean)(C.int(1)))
	} else {
		C.messaging_menu_app_append_message(app.instance, msg.instance, _id, (C.gboolean)(C.int(0)))
	}
	free(_id)
}

func (app *MessagingMenuApp) GetMessage(id string) MessagingMenuMessage {
	var _id = gchar(id)
	var msg = MessagingMenuMessage{C.messaging_menu_app_get_message(app.instance, _id)}
	free(_id)
	return msg
}

func (app *MessagingMenuApp) RemoveMessage(msg MessagingMenuMessage) {
	C.messaging_menu_app_remove_message(app.instance, msg.instance)
}

func (app *MessagingMenuApp) RemoveMessageById(id string) {
	var _id = gchar(id)
	C.messaging_menu_app_remove_message_by_id(app.instance, _id)
	free(_id)
}

func (msg *MessagingMenuMessage) GetId() string {
	var _id = (*C.char)(C.messaging_menu_message_get_id(msg.instance))
	var id = C.GoString(_id)
	// g_free this time because it's allocated via g_malloc
	C.g_free((C.gpointer)(_id))
	return id
}

func (msg *MessagingMenuMessage) GetIcon() *C.GIcon {
	return C.messaging_menu_message_get_icon(msg.instance)
}

func (msg *MessagingMenuMessage) GetTitle() string {
	var _title = C.messaging_menu_message_get_title(msg.instance)
	var title = C.GoString((*C.char)(_title))
	// g_free this time because it's allocated via g_malloc
	C.g_free((C.gpointer)(_title))
	return title
}

func (msg *MessagingMenuMessage) GetSubtitle() string {
	var _subtitle = C.messaging_menu_message_get_subtitle(msg.instance)
	var subtitle = C.GoString((*C.char)(_subtitle))
	// g_free this time because it's allocated via g_malloc
	C.g_free((C.gpointer)(_subtitle))
	return subtitle
}

func (msg *MessagingMenuMessage) GetBody() string {
	var _body = C.messaging_menu_message_get_body(msg.instance)
	var body = C.GoString((*C.char)(_body))
	// g_free this time because it's allocated via g_malloc
	C.g_free((C.gpointer)(_body))
	return body
}

func (msg *MessagingMenuMessage) GetTime() int {
	return int((C.int)(C.messaging_menu_message_get_time(msg.instance)))
}

func (msg *MessagingMenuMessage) GetDrawsAttention() bool {
	return int((C.int)(C.messaging_menu_message_get_draws_attention(msg.instance))) != 0
}

func (msg *MessagingMenuMessage) SetDrawsAttention(draws_attention bool) {
	if draws_attention { // FIXME: how to convert from bool to int?
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

// Connect one of the messagingMenuApp's signals to a C function
func (app *MessagingMenuApp) Connect(detailed_signal string, callback unsafe.Pointer, gobject C.gpointer) {
	var _detailed_signal = gchar(detailed_signal)
	C.g_signal_connect_object(
		(C.gpointer)(app.instance),
		_detailed_signal,
		(C.GCallback)(callback),
		gobject,
		C.G_CONNECT_AFTER)
	free(_detailed_signal)
}

// Connect one of the MessagingMenuMessage's signals to a C function
func (msg *MessagingMenuMessage) Connect(detailed_signal string, callback unsafe.Pointer, gobject C.gpointer) {
	var _detailed_signal = gchar(detailed_signal)
	C.g_signal_connect_object(
		(C.gpointer)(msg.instance),
		_detailed_signal,
		(C.GCallback)(callback),
		gobject,
		C.G_CONNECT_AFTER)
	free(_detailed_signal)
}

// EnterMainLoop is a temporarty hack pending real event loop integration from tvoss.
// call it so that the glib event loop handles sending messages and triggering signals.
func EnterMainLoop() {
	var loop = C.g_main_loop_new(nil, 0)
	go C.g_main_loop_run(loop)
}
