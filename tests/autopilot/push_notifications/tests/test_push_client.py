# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.

"""Tests for Push Notifications client"""

from __future__ import absolute_import

from testtools.matchers import Equals
from push_notifications.tests import PushNotificationTestBase


class TestPushClient(PushNotificationTestBase):
    """ Tests a Push notification can be sent and received """

    def _validate_response(self, response, expected_status_code=200):
        """
        Validate the received response status code against expected code
        """
        self.assertThat(response.status, Equals(expected_status_code))

    def test_broadcast_push_notification(self):
        """
        Positive test case to send a valid broadcast push notification
        to the client and validate that a notification message is displayed
        """
        # create a copy of the device's build info
        msg_data = self.create_notification_data_copy()
        # increment the build number to trigger an update
        msg_data.inc_build_number()
        # create message based on the data
        msg = self.create_push_message(data=msg_data.json())
        # send the notification message to the server and check response
        response = self.send_push_broadcast_notification(msg.json())
        self._validate_response(response)

        # TODO validate that message is received on client

    def test_expired_broadcast_push_notification(self):
        """
        Send an expired broadcast notification message to server
        """
        msg = self.create_push_message(expire_after=self.get_past_iso_time())
        response = self.send_push_broadcast_notification(msg.json())
        # 400 status is received for an expired message
        self._validate_response(response, expected_status_code='400')

        # TODO validate that message is not received on client

    def test_near_expiry_broadcast_push_notification(self):
        """
        Send a broadcast message with a short validity time
        """
        msg = self.create_push_message(
            expire_after=self.get_near_future_iso_time())
        response = self.send_push_broadcast_notification(msg.json())
        self._validate_response(response)

        # TODO validate that message is received on client

    def test_just_expired_broadcast_push_notification(self):
        """
        Send a broadcast message which has just expired
        """
        msg = self.create_push_message(
            expire_after=self.get_near_past_iso_time())
        response = self.send_push_broadcast_notification(msg.json())
        self._validate_response(response)

        # TODO validate that message is not received on client
