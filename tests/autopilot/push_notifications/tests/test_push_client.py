# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.

"""Tests for Push Notifications client"""

from testtools.matchers import Equals
from push_notifications.tests import PushNotificationTestBase
from autopilot.introspection import dbus
import time


class TestPushClient(PushNotificationTestBase):
    """ Tests a Push notification can be sent and received """

    DEFAULT_DISPLAY_MESSAGE = 'There\'s an updated system image.'

    def _validate_response(self, response, expected_status_code=200):
        """
        Validate the received response status code against expected code
        """
        self.assertThat(response.status, Equals(expected_status_code))

    def _validate_notification_displayed(self,
                                         msg_text=DEFAULT_DISPLAY_MESSAGE):
        """
        Validate that the notification is displayed
        Return the dialog object
        """
        dialog = self.main_window.wait_select_single(
            'Notification', objectName='notification1')
        self.assertEqual(msg_text, dialog.summary)
        return dialog

    def _validate_notification_not_displayed(self, wait=True):
        """
        Validate that the notification is not displayed
        If wait is True then wait for default timeout period
        If wait is False then do not wait at all
        """
        found = True
        try:
            if wait is True:
                self.main_window.wait_select_single(
                    'Notification', objectName='notification1')
            else:
                self.main_window.select_single(
                    'Notification', objectName='notification1')
        except dbus.StateNotFoundError:
            found = False
        self.assertFalse(found)

    def _send_valid_push_message(self):
        """
        Send a valid push message which should trigger a notification
        to be displayed on the client
        """
        # create a copy of the device's build info
        msg_data = self.create_notification_data_copy()
        # increment the build number to trigger an update
        msg_data.inc_build_number()
        # create message based on the data
        msg = self.push_helper.create_push_message(data=msg_data.json())
        # send the notification message to the server and check response
        response = self.push_helper.send_push_broadcast_notification(
            msg.json(), self.test_config.server_listener_addr)
        self._validate_response(response)

    def _validate_and_dismiss_notification_dialog(self, message):
        """
        Validate a notification dialog is displayed and dismiss it
        """
        # validate dialog
        dialog = self._validate_notification_displayed(message)
        # press dialog to dismiss
        self._press_notification_dialog(dialog)
        # check the dialog is no longer displayed
        self._validate_notification_not_displayed(wait=False)

    def _press_notification_dialog(self, dialog):
        """
        Press the dialog to dismiss it
        """
        self.touch.tap_object(dialog)

    def test_broadcast_push_notification_screen_off(self):
        """
        Send a push message whilst the device's screen is turned off
        Notification should still be displayed when it is turned on
        """
        # Turn display off
        self.press_power_button()
        # send message
        self._send_valid_push_message()
        # wait before turning screen on
        time.sleep(5)
        # Turn display on
        self.press_power_button()

        self._validate_and_dismiss_notification_dialog(
            self.DEFAULT_DISPLAY_MESSAGE)

    def test_broadcast_push_notification_locked_greeter(self):
        """
        Positive test case to send a valid broadcast push notification
        to the client and validate that a notification message is displayed
        whist the greeter screen is displayed
        """
        # Assumes greeter starts in locked state
        self._send_valid_push_message()
        self._validate_and_dismiss_notification_dialog(
            self.DEFAULT_DISPLAY_MESSAGE)

    def test_broadcast_push_notification(self):
        """
        Positive test case to send a valid broadcast push notification
        to the client and validate that a notification message is displayed
        """
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        # send message
        self._send_valid_push_message()
        self._validate_and_dismiss_notification_dialog(
            self.DEFAULT_DISPLAY_MESSAGE)

    def test_expired_broadcast_push_notification(self):
        """
        Send an expired broadcast notification message to server
        """
        self.unlock_greeter()
        msg_data = self.create_notification_data_copy()
        msg_data.inc_build_number()
        msg = self.push_helper.create_push_message(
            data=msg_data.json(),
            expire_after=self.push_helper.get_past_iso_time())
        response = self.push_helper.send_push_broadcast_notification(
            msg.json(),
            self.test_config.server_listener_addr)
        # 400 status is received for an expired message
        self._validate_response(response, expected_status_code=400)
        # validate no notification is displayed
        self._validate_notification_not_displayed()

    def test_older_version_broadcast_push_notification(self):
        """
        Send an old version broadcast notification message to server
        """
        self.unlock_greeter()
        msg_data = self.create_notification_data_copy()
        msg_data.dec_build_number()
        msg = self.push_helper.create_push_message(data=msg_data.json())
        response = self.push_helper.send_push_broadcast_notification(
            msg.json(),
            self.test_config.server_listener_addr)
        self._validate_response(response)
        # validate no notification is displayed
        self._validate_notification_not_displayed()

    def test_equal_version_broadcast_push_notification(self):
        """
        Send an equal version broadcast notification message to server
        """
        self.unlock_greeter()
        msg_data = self.create_notification_data_copy()
        msg = self.push_helper.create_push_message(data=msg_data.json())
        response = self.push_helper.send_push_broadcast_notification(
            msg.json(),
            self.test_config.server_listener_addr)
        self._validate_response(response)
        # validate no notification is displayed
        self._validate_notification_not_displayed()
