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
package cmessaging

/*
#include <glib.h>
#include <messaging-menu/messaging-menu-app.h>
#include <messaging-menu/messaging-menu-message.h>

static void activate_cb(MessagingMenuMessage* msg, gchar* action, GVariant* parameter, gpointer obj) {
    handleActivate(action, messaging_menu_message_get_id(msg), obj);
}

void add_notification(const gchar* app_id, const gchar* notification_id,
          const gchar* icon_path, const gchar* summary, const gchar* body,
          guint64 timestamp, const gchar** actions, gpointer obj) {
    MessagingMenuApp* app = messaging_menu_app_new(app_id);
    messaging_menu_app_register(app);
    GIcon* icon = g_icon_new_for_string(icon_path, NULL);
    messaging_menu_app_append_source(app, "postal", icon, "Postal");

    MessagingMenuMessage* msg = messaging_menu_message_new(notification_id, icon, "Title",
                                                           "subtitle", "this is body",
                                                           g_date_time_to_unix (g_date_time_new_now_utc ()));
    // unity8 support for actions in the messaging menu is strange. Not doing that for now.
    messaging_menu_app_append_message(app, msg, "postal", TRUE);

    g_signal_connect(msg, "activate", G_CALLBACK(activate_cb), obj);
}

*/
import "C"
