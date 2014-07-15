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
package cmessaging

/*
#cgo pkg-config: messaging-menu

#include <glib.h>

void add_notification(const gchar* desktop_id, const gchar* notification_id,
          const gchar* icon_path, const gchar* summary, const gchar* body,
          gint64 timestamp, const gchar** actions, const size_t actions_len, gpointer obj);
*/
import "C"
import "unsafe"

import (
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/messaging/reply"
)

func gchar(s string) *C.gchar {
	return (*C.gchar)(C.CString(s))
}

func gfree(s *C.gchar) {
	C.g_free((C.gpointer)(s))
}

//export handleActivate
func handleActivate(action *C.char, notification *C.char, ch *chan *reply.MMActionReply) {
	mmar := &reply.MMActionReply{Notification: C.GoString(notification), Action: C.GoString(action)}
	*ch <- mmar
}

func AddNotification(desktopId string, notificationId string, card *launch_helper.Card, actions []string, ch chan *reply.MMActionReply) {
	desktop_id := gchar(desktopId)
	defer gfree(desktop_id)

	notification_id := gchar(notificationId)
	defer gfree(notification_id)

	icon_path := gchar(card.Icon)
	defer gfree(icon_path)

	summary := gchar(card.Summary)
	defer gfree(summary)

	body := gchar(card.Body)
	defer gfree(body)

	// TODO: build the action_list
	var action_list_arg **C.gchar = nil
	var action_list_size int = 0
	if len(actions) > 0 {
		action_list := make([]*C.gchar, len(actions)+1)
		for i, action := range actions {
			c_action := gchar(action)
			defer gfree(c_action)
			action_list[i] = c_action
		}
		action_list_arg = (**C.gchar)(unsafe.Pointer(&action_list[0]))
		action_list_size = len(action_list)
	}

	timestamp := (C.gint64)(int64(card.Timestamp) * 1000000)

	C.add_notification(desktop_id, notification_id, icon_path, summary, body, timestamp, action_list_arg, C.size_t(action_list_size), (C.gpointer)(&ch))
}

func init() {
	go C.g_main_loop_run(C.g_main_loop_new(nil, C.FALSE))
}
