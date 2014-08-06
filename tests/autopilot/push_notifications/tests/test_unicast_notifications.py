# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
#
# Push Notifications Autopilot Test Suite
# Copyright (C) 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.
#

"""Tests unicast push notifications sent to the client"""

import os

from push_notifications.tests import PushNotificationTestBase


class TestPushClientUnicast(PushNotificationTestBase):
    """Test cases for unicast push notifications."""

    DEFAULT_DISPLAY_MESSAGE = 'Look!'

    scenarios = [('click_app_with_version',
                  dict(app_name="com.ubuntu.developer.webapps.webapp-twitter_webapp-twitter",
                       appid=None, path=None,
                       desktop_dir="~/.local/share/applications/",
                       icon="twitter",
                       launcher_idx=5)),
                 ('click_app',
                  dict(app_name="com.ubuntu.developer.webapps.webapp-twitter_webapp-twitter",
                       appid="com.ubuntu.developer.webapps.webapp-twitter_webapp-twitter",
                       path="com_2eubuntu_2edeveloper_2ewebapps_2ewebapp_2dtwitter",
                       desktop_dir="~/.local/share/applications/",
                       icon="twitter",
                       launcher_idx=5)),
                 ('legacy_app',
                  dict(app_name="messaging-app",
                       appid="_messaging-app",
                       path="_",
                       desktop_dir="/usr/share/applications/",
                       icon="messages-app",
                       launcher_idx=1))]

    def setUp(self):
        super(TestPushClientUnicast, self).setUp()
        # only for the click_app_with_version scenario
        if self.path is None and self.appid is None:
            self.appid, self.path = self._get_click_appid_and_path()
        self.token = self.push_helper.register(self.path, self.appid)

    def _get_click_appid_and_path(self):
        """Return the click appid including the version and dbus path."""
        # get appid with version from the .desktop file list.
        files = os.listdir(os.path.expanduser(self.desktop_dir))
        for fname in files:
            if fname.startswith(self.app_name) and \
                    fname.endswith(".desktop"):
                # remove .desktop extension, only need the name.
                appid = os.path.splitext(fname)[0]
                path = appid.split("_")[0]
                path = path.replace(".", "_2e").replace("-", "_2d")
                return appid, path
        return self.appid, self.path

    def test_unicast_push_notification_persistent(self):
        """Send a persistent unicast push notification.

        Notification should be displayed in the incoming indicator.

        """
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        # send message
        self.send_unicast_notification(persist=True, popup=False,
                                       icon=self.icon)
        self.validate_mmu_notification("A unicast message", "Look!")

    def get_running_app_launcher_icon(self):
        launcher = self.main_window.get_launcher()
        return launcher.select_single(
            'LauncherDelegate',
            objectName='launcherDelegate%d' % self.launcher_idx)

    def test_unicast_push_notification_emblem_count(self):
        """Send a emblem-counter enabled unicast push notification.

        Notification should be displayed at the dash/emblem.

        """
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        # open the app, only if isn't by default in the launcher_id
        if self.launcher_idx >= 4:
            try:
                self.launch_upstart_application(
                    self._get_click_appid_and_path()[0])
            except Exception:
                # ignore dbus instrospection errors
                pass
        # move the app to the background
        self.main_window.show_dash_swiping()
        # check the icon has no emblems
        app_icon = self.get_running_app_launcher_icon()
        self.assertEqual(app_icon.count, 0)
        # send message, only showing emblem counter
        emblem_counter = {'count': 42, 'visible': True}
        self.send_unicast_notification(persist=False, popup=False,
                                       icon=self.icon,
                                       emblem_counter=emblem_counter)
        # show the dash and check the emblem count.
        self.main_window.show_dash_swiping()
        # check there is a emblem count == 2
        app_icon = self.get_running_app_launcher_icon()
        self.assertEqual(app_icon.count, emblem_counter['count'])

    def test_unicast_push_notification_locked_greeter(self):
        """Send a push notification while in the greeter scrren.

        The notification should be displayed on top of the greeter.
        """
        # Assumes greeter starts in locked state
        self.send_unicast_notification(summary="Locked greeter",
                                       icon=self.icon)
        self.validate_and_dismiss_notification_dialog("Locked greeter")

    def test_unicast_push_notification(self):
        """Send a push notificationn and validate it's displayed."""
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        self.send_unicast_notification(icon=self.icon)
        self.validate_and_dismiss_notification_dialog(
            self.DEFAULT_DISPLAY_MESSAGE)

    def test_unicast_push_notification_on_connect(self):
        """Send a unicast notification whilst the push client is disconnected.

        Then reconnect and ensure message is displayed.
        """

        # Assumes greeter starts in locked state
        self.unlock_greeter()
        self.push_client_controller.stop_push_client()
        self.send_unicast_notification(icon=self.icon)
        self.push_client_controller.start_push_client()
        self.validate_and_dismiss_notification_dialog(
            self.DEFAULT_DISPLAY_MESSAGE)

    def test_expired_unicast_push_notification(self):
        """Send an expired unicast notification message to server."""
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        # create notification message using past expiry time
        expire_on = self.push_helper.get_past_iso_time()
        # XXX: build a unicast message
        notif = {"notification": {"card":
                                  {"icon": "messages-app",
                                   "summary": "A summary",
                                   "body": "The body of the msg.",
                                   "popup": True,
                                   "actions": []
                                   }
                                  }
                 }
        data = {'token': self.token,
                'data': notif,
                'appid': self.appid,
                'expire_on': expire_on}
        # send message
        response = self.push_helper.send_unicast(
            data, self.test_config.server_listener_addr)
        # 400 status is received for an expired message
        self.validate_response(response, expected_status_code=400)
        # validate no notification is displayed
        self.validate_notification_not_displayed()

