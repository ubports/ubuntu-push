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

void add_notification(const gchar* app_id, const gchar* notification_id,
          const gchar* icon_path, const gchar* summary, const gchar* body,
          guint64 timestamp, const gchar** actions, gpointer obj);
*/
import "C"

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

func AddNotification(appId string, notificationId string, card *launch_helper.Card, ch chan *reply.MMActionReply) {
	app_id := gchar(appId)
	defer gfree(app_id)

	notification_id := gchar(notificationId)
	defer gfree(notification_id)

	icon_path := gchar(card.Icon)
	defer gfree(icon_path)

	summary := gchar(card.Summary)
	defer gfree(summary)

	body := gchar(card.Body)
	defer gfree(body)

	C.add_notification(app_id, notification_id, icon_path, summary, body, (C.guint64)(card.Timestamp), nil, (C.gpointer)(&ch))
}

func init() {
	go C.g_main_loop_run(C.g_main_loop_new(nil, C.FALSE))
}
