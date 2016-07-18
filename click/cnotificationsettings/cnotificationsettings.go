/*
 Copyright 2016 Canonical Ltd.

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

// Package cnotificationsettings accesses the g_settings notification settings

package cnotificationsettings

/*
#cgo pkg-config: gio-unix-2.0
#cgo pkg-config: glib-2.0

#include <stdlib.h>
#include <gio/gio.h>

#define NOTIFICATION_APPS_SETTINGS_SCHEMA_ID "com.ubuntu.notifications.settings.applications"
#define VIBRATE_SILENT_MODE_KEY "vibrate-silent-mode"
#define NOTIFICATION_SETTINGS_SCHEMA_ID "com.ubuntu.notifications.settings"
#define SETTINGS_BASE_PATH "/com/ubuntu/NotificationSettings/"
#define ENABLE_NOTIFICATIONS_KEY "enable-notifications"
#define USE_SOUNDS_NOTIFICATIONS_KEY "use-sounds-notifications"
#define USE_VIBRATIONS_NOTIFICATIONS_KEY "use-vibrations-notifications"
#define USE_BUBBLES_NOTIFICATIONS_KEY "use-bubbles-notifications"
#define USE_LIST_NOTIFICATIONS_KEY "use-list-notifications"

GSettings* get_settings() {
    // Check if GSettings schema exists
    GSettingsSchemaSource *source = g_settings_schema_source_get_default();
    if (!g_settings_schema_source_lookup(source, NOTIFICATION_APPS_SETTINGS_SCHEMA_ID, TRUE)) {
        return NULL;
    }

    return g_settings_new(NOTIFICATION_APPS_SETTINGS_SCHEMA_ID);
}

int vibrate_in_silent_mode() {
    GSettings *settings = NULL;
    int vibrateInSilentMode = 0;

    settings = get_settings();
    if (!settings) {
        return -1;
    }

    if (g_settings_get_boolean(settings, VIBRATE_SILENT_MODE_KEY)) {
        vibrateInSilentMode = 1;
    }

    g_object_unref(settings);
    return vibrateInSilentMode;
}

GSettings* get_settings_for_app(const char *pkgname, const char *appname) {
    GSettings *settings = NULL;
    gchar *path;

    // Check if GSettings schema exists
    GSettingsSchemaSource *source = g_settings_schema_source_get_default();
    if (!g_settings_schema_source_lookup(source, NOTIFICATION_SETTINGS_SCHEMA_ID, TRUE)) {
        return NULL;
    }

    // Define notifications settings GSettings path
    if (pkgname == "") {
        // Use "dpkg" as package name for legacy apps
        path = g_strconcat(SETTINGS_BASE_PATH, "dpkg/", appname, "/", NULL);
    } else {
        path = g_strconcat(SETTINGS_BASE_PATH, pkgname, "/", appname, "/", NULL);
    }

    settings = g_settings_new_with_path(NOTIFICATION_SETTINGS_SCHEMA_ID, path);
    g_free(path);

    return settings;
}

int are_notifications_enabled(const char *pkgname, const char *appname) {
    GSettings *notificationSettings = NULL;
    int enableNotifications = 0;

    notificationSettings = get_settings_for_app(pkgname, appname);
    if (!notificationSettings) {
        return -1;
    }

    if (g_settings_get_boolean(notificationSettings, ENABLE_NOTIFICATIONS_KEY)) {
        enableNotifications = 1;
    }

    g_object_unref(notificationSettings);
    return enableNotifications;
}

int can_use_sounds_notify(const char *pkgname, const char *appname) {
    GSettings *notificationSettings = NULL;
    int soundsNotify = 0;

    notificationSettings = get_settings_for_app(pkgname, appname);
    if (!notificationSettings) {
        return -1;
    }

    if (g_settings_get_boolean(notificationSettings, USE_SOUNDS_NOTIFICATIONS_KEY)) {
        soundsNotify = 1;
    }

    g_object_unref(notificationSettings);
    return soundsNotify;
}

int can_use_vibrations_notify(const char *pkgname, const char *appname) {
    GSettings *notificationSettings = NULL;
    int vibrationsNotify = 0;

    notificationSettings = get_settings_for_app(pkgname, appname);
    if (!notificationSettings) {
        return -1;
    }

    if (g_settings_get_boolean(notificationSettings, USE_VIBRATIONS_NOTIFICATIONS_KEY)) {
        vibrationsNotify = 1;
    }

    g_object_unref(notificationSettings);
    return vibrationsNotify;
}

int can_use_bubbles_notify(const char *pkgname, const char *appname) {
    GSettings *notificationSettings = NULL;
    int bubblesNotify = 0;

    notificationSettings = get_settings_for_app(pkgname, appname);
    if (!notificationSettings) {
        return -1;
    }

    if (g_settings_get_boolean(notificationSettings, USE_BUBBLES_NOTIFICATIONS_KEY)) {
        bubblesNotify = 1;
    }

    g_object_unref(notificationSettings);
    return bubblesNotify;
}

int can_use_list_notify(const char *pkgname, const char *appname) {
    GSettings *notificationSettings = NULL;
    int listNotify = 0;

    notificationSettings = get_settings_for_app(pkgname, appname);
    if (!notificationSettings) {
        return -1;
    }

    if (g_settings_get_boolean(notificationSettings, USE_LIST_NOTIFICATIONS_KEY)) {
        listNotify = 1;
    }

    g_object_unref(notificationSettings);
    return listNotify;
}
*/
import "C"

import (
	"unsafe"

	"launchpad.net/ubuntu-push/click"
)

// VibrateInSilentMode returns true if applications can use vibrations notify when in silent mode
func VibrateInSilentMode() bool {
	return C.vibrate_in_silent_mode() != 0
}

// AreNotificationsEnabled returns true if the application is marked on gsettings to use notifications
func AreNotificationsEnabled(app *click.AppId) bool {
	pkgname := C.CString(app.Package)
	appname := C.CString(app.Application)
	defer C.free(unsafe.Pointer(pkgname))
	defer C.free(unsafe.Pointer(appname))
	return C.are_notifications_enabled(pkgname, appname) != 0
}

// CanUseSoundsNotify returns true if the application is marked on gsettings to use sounds notify
func CanUseSoundsNotify(app *click.AppId) bool {
	pkgname := C.CString(app.Package)
	appname := C.CString(app.Application)
	defer C.free(unsafe.Pointer(pkgname))
	defer C.free(unsafe.Pointer(appname))
	return C.can_use_sounds_notify(pkgname, appname) != 0
}

// CanUseVibrationsNotify returns true if the application is marked on gsettings to use vibrations notify
func CanUseVibrationsNotify(app *click.AppId) bool {
	pkgname := C.CString(app.Package)
	appname := C.CString(app.Application)
	defer C.free(unsafe.Pointer(pkgname))
	defer C.free(unsafe.Pointer(appname))
	return C.can_use_vibrations_notify(pkgname, appname) != 0
}

// CanUseBubblesNotify returns true if the application is marked on gsettings to use bubbles notify
func CanUseBubblesNotify(app *click.AppId) bool {
	pkgname := C.CString(app.Package)
	appname := C.CString(app.Application)
	defer C.free(unsafe.Pointer(pkgname))
	defer C.free(unsafe.Pointer(appname))
	return C.can_use_bubbles_notify(pkgname, appname) != 0
}

// CanUseListNotify returns true if the application is marked on gsettings to use list notificy
func CanUseListNotify(app *click.AppId) bool {
	pkgname := C.CString(app.Package)
	appname := C.CString(app.Application)
	defer C.free(unsafe.Pointer(pkgname))
	defer C.free(unsafe.Pointer(appname))
	return C.can_use_list_notify(pkgname, appname) != 0
}
