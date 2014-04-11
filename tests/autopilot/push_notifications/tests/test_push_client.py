# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.

"""Tests for Push Notifications client"""

from __future__ import absolute_import

from testtools.matchers import Equals
from autopilot.matchers import Eventually
from autopilot.introspection import dbus

from autopilot.testcase import AutopilotTestCase

from push_notifications.tests import PushNotificationTestBase
from push_notifications.tests import PushNotificationMessage


class TestPushClient(PushNotificationTestBase):
    """ Tests a Push notification can be sent and received """

    def _validate_response(self, response, expected_status_code='200'):
        """
        Validate the received response status code against expected code
        """
        status = response[0]['status']
        self.assertThat(status, Equals(expected_status_code))

    def test_broadcast_push_notification(self):
        """
        Positive test case to send a valid broadcast push notification
        to the client and validate that a notification message is displayed
        """
        msg = self.create_push_message()
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
        msg = self.create_push_message(expire_after=self.get_near_future_iso_time())
        response = self.send_push_broadcast_notification(msg.json())
        self._validate_response(response)

        # TODO validate that message is received on client

    def test_just_expired_broadcast_push_notification(self):
        """
        Send a broadcast message which has just expired
        """
        msg = self.create_push_message(expire_after=self.get_near_past_iso_time())
        response = self.send_push_broadcast_notification(msg.json())
        self._validate_response(response)

        # TODO validate that message is not received on client
