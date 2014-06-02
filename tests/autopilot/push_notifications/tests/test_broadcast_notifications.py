# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.

"""Tests broadcast push notifications sent to the client"""

import time
from push_notifications.tests import PushNotificationTestBase


class TestPushClientBroadcast(PushNotificationTestBase):
    """
    Test cases for broadcast push notifications
    """

    DEFAULT_DISPLAY_MESSAGE = 'There\'s an updated system image.'

    def test_broadcast_push_notification_screen_off(self):
        """
        Send a push message whilst the device's screen is turned off
        Notification should still be displayed when it is turned on
        """
        # Assumes greeter starts in locked state
        # Turn display off
        self.press_power_button()
        # send message
        self.send_push_broadcast_message()
        # wait before turning screen on
        time.sleep(2)
        # Turn display on
        self.press_power_button()
        self.validate_and_dismiss_notification_dialog(
            self.DEFAULT_DISPLAY_MESSAGE)

    def test_broadcast_push_notification_locked_greeter(self):
        """
        Positive test case to send a valid broadcast push notification
        to the client and validate that a notification message is displayed
        whist the greeter screen is displayed
        """
        # Assumes greeter starts in locked state
        self.send_push_broadcast_message()
        self.validate_and_dismiss_notification_dialog(
            self.DEFAULT_DISPLAY_MESSAGE)

    def test_broadcast_push_notification(self):
        """
        Positive test case to send a valid broadcast push notification
        to the client and validate that a notification message is displayed
        """
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        # send message
        self.send_push_broadcast_message()
        self.validate_and_dismiss_notification_dialog(
            self.DEFAULT_DISPLAY_MESSAGE)

    def test_expired_broadcast_push_notification(self):
        """
        Send an expired broadcast notification message to server
        """
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        # create notification message using past expiry time
        device_info = self.create_device_info_copy()
        device_info.inc_build_number()
        push_msg = self.push_helper.create_push_message(
            data=device_info.json(),
            expire_after=self.push_helper.get_past_iso_time())
        # send message
        response = self.push_helper.send_push_broadcast_notification(
            push_msg.json(),
            self.test_config.server_listener_addr)
        # 400 status is received for an expired message
        self.validate_response(response, expected_status_code=400)
        # validate no notification is displayed
        self.validate_notification_not_displayed()

    def test_older_version_broadcast_push_notification(self):
        """
        Send an old version broadcast notification message to server
        """
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        # create notification message using previous build number
        device_info = self.create_device_info_copy()
        device_info.dec_build_number()
        push_msg = self.push_helper.create_push_message(
            data=device_info.json())
        response = self.push_helper.send_push_broadcast_notification(
            push_msg.json(),
            self.test_config.server_listener_addr)
        self.validate_response(response)
        # validate no notification is displayed
        self.validate_notification_not_displayed()

    def test_equal_version_broadcast_push_notification(self):
        """
        Send an equal version broadcast notification message to server
        """
        # Assumes greeter starts in locked state
        self.unlock_greeter()
        # create notification message using equal build number
        device_info = self.create_device_info_copy()
        push_msg = self.push_helper.create_push_message(
            data=device_info.json())
        response = self.push_helper.send_push_broadcast_notification(
            push_msg.json(),
            self.test_config.server_listener_addr)
        self.validate_response(response)
        # validate no notification is displayed
        self.validate_notification_not_displayed()
