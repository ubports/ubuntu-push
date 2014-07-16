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

package cual

// this is a .go to work around limitations in dh-golang

/*
#include <ubuntu-app-launch.h>
#include <glib.h>

#define HELPER_ERROR g_quark_from_static_string ("cgo-ual-helper-error-quark")

static void observer_of_stop (const gchar * app_id, const gchar * instance_id, const gchar * helper_type, gpointer user_data) {
    helperDone (user_data, instance_id);
}

char* launch(gchar* app_id, gchar* exec, gchar* f1, gchar* f2, gpointer p) {
    const gchar* uris[4] = {exec, f1, f2, NULL};
    gchar* iid = ubuntu_app_launch_start_multiple_helper ("push-helper", app_id, uris);
    g_free (app_id);
    g_free (exec);
    g_free (f1);
    g_free (f2);
    return iid;
}

gboolean add_observer(gpointer p) {
    return ubuntu_app_launch_observer_add_helper_stop(observer_of_stop, "push-helper", p);
}

gboolean remove_observer(gpointer p) {
    return ubuntu_app_launch_observer_delete_helper_stop(observer_of_stop, "push-helper", p);
}

void stop(gchar* app_id, gchar* iid) {
    ubuntu_app_launch_stop_multiple_helper ("push-helper", app_id, iid);
    g_free (app_id);
    g_free (iid);
}
*/
import "C"
