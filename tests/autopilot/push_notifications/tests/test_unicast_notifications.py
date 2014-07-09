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
import time

from push_notifications.tests import PushNotificationTestBase


class TestPushClientUnicast(PushNotificationTestBase):
    """Test cases for unicast push notifications."""

    DEFAULT_DISPLAY_MESSAGE = 'Look!'

    def setUp(self):
        super(TestPushClientUnicast, self).setUp()
        # get calculator appid with version (needed for push to use mmu)
        # from the .desktop file list.
        files = os.listdir(os.path.expanduser("~/.local/share/applications/"))
        for fname in files:
            if fname.startswith('com.ubuntu.calculator_calculator') and \
                    fname.endswith(".desktop"):
                # remove .desktop extension, only need the name.
                self.appid = os.path.splitext(fname)[0]
                break
        else:
            self.appid = "com.ubuntu.calculator_calculator"
        self.token = self.push_helper.register(appid=self.appid)

    def test_unicast_push_notification_persistent(self):
        """Send a persistent unicast push notification.

        Notification should be displayed in the incoming indicator.

        """
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        # send message
        self.send_unicast_notification(persist=True)
        # swipe down and show the incomming page
        messaging = self.get_messaging_menu()
        # get the notification and check the body and title.
        menuItem0 = messaging.select_single('QQuickLoader',
                                            objectName='menuItem0')
        hmh = menuItem0.select_single('HeroMessageHeader')
        body = hmh.select_single("Label", objectName='body')
        self.assertEqual(body.text, 'A unicast message')
        title = hmh.select_single("Label", objectName='title')
        self.assertEqual(title.text, 'Look!')

    def swipe_screen_from_left(self):
        width = self.main_window.width
        height = self.main_window.height
        start_x = 50
        start_y = int(height/2)
        end_x = int(width/2)
        end_y = width
        self.touch.drag(start_x, start_y, end_x, end_y)

    def get_running_app_launcher_icon(self):
        launcher = self.main_window.get_launcher()
        return launcher.select_single('LauncherDelegate',
                                      objectName='launcherDelegate4')

    def test_unicast_push_notification_emblem_count(self):
        """Send a emblem-counter enabled unicast push notification.

        Notification should be displayed at the dash/emblem.

        """
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        # open self.appid app
        try:
            self.launch_upstart_application(self.appid)
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
        self.send_unicast_notification(summary="Locked greeter")
        self.validate_and_dismiss_notification_dialog("Locked greeter")

    def test_unicast_push_notification(self):
        """Send a push notificationn and validate it's displayed."""
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        self.send_unicast_notification()
        self.validate_and_dismiss_notification_dialog(
            self.DEFAULT_DISPLAY_MESSAGE)

    def test_unicast_push_notification_on_connect(self):
        """Send a unicast notification whilst the push client is disconnected.

        Then reconnect and ensure message is displayed.
        """

        # Assumes greeter starts in locked state
        self.unlock_greeter()
        self.push_client_controller.stop_push_client()
        self.send_unicast_notification()
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

