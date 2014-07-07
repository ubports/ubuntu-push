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

import time

from push_notifications.tests import PushNotificationTestBase


class TestPushClientUnicast(PushNotificationTestBase):
    """Test cases for unicast push notifications."""

    DEFAULT_DISPLAY_MESSAGE = 'Look!'

    def setUp(self):
        super(TestPushClientUnicast, self).setUp()
        self.appid = "com.ubuntu.calculator_current"
        self.token = self.push_helper.register(appid=self.appid)

    def test_unicast_push_notification_incoming_screen_off(self):
        """Send a push message whilst the device's screen is turned off.

        Notification should still be displayed in the incoming menu
        when it is turned on.

        """
        # Assumes greeter starts in locked state
        # Turn display off
        self.press_power_button()
        # send message
        self.send_unicast_notification(persist=True, popup=True)
        # wait before turning screen on
        time.sleep(2)
        # Turn display on
        self.press_power_button()
        self.unlock_greeter()
        messaging = self.get_messaging_menu()
        label = messaging.select_single('Label', objectName='emptyLabel')
        self.assertNotEqual(label.text, "Empty!", "The incoming list is empty")

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

