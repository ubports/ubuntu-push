# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.

"""push-notifications autopilot tests."""

import copy

import evdev

from autopilot.introspection import dbus
from autopilot.matchers import Eventually
from push_notifications import config as push_config
import push_notifications.helpers.push_notifications_helper as push_helper
from testtools.matchers import Equals, NotEquals
from unity8.shell.tests import UnityTestCase


class PushNotificationTestBase(UnityTestCase):
    """
    Base class for push notification test cases
    """

    @classmethod
    def setUpClass(cls):
        """
        Executed once before all the tests run
        Restart the push client using the test config
        """
        test_config = push_helper.PushClientConfig.read_config(
            push_config.get_config_file())
        push_client_controller = push_helper.PushClientController()
        push_client_controller.restart_push_client_using_config(test_config)

    @classmethod
    def tearDownClass(cls):
        """
        Executed once after all tests have completed
        Reset the push client to use the device's original config
        """
        push_client_controller = push_helper.PushClientController()
        push_client_controller.restart_push_client_using_config(None)

    def setUp(self):
        """
        Setup phase executed before each test
        """
        # setup
        super(PushNotificationTestBase, self).setUp()

        # read and store the test config data
        self.test_config = push_helper.PushClientConfig.read_config(
            push_config.get_config_file())
        # create a push helper object which will do all the message sending
        self.push_helper = push_helper.PushNotificationHelper()
        # get and store device and build info
        self.device_info = self.push_helper.get_device_info()
        # start unity8
        self._qml_mock_enabled = False
        self._data_dirs_mock_enabled = False
        self.unity = self.launch_unity()
        # dismiss any outstanding dialog
        self.dismiss_outstanding_dialog()

    def create_device_info_copy(self):
        """
        Return a copy of the device's model and build info
        :return: DeviceNotificationData object containging device's model
                 and build info
        """
        return copy.deepcopy(self.device_info)

    def press_power_button(self):
        """
        Simulate a power key press event
        """
        uinput = evdev.UInput(name='push-autopilot-power-button',
                              devnode='/dev/autopilot-uinput')
        # One press and release to turn screen off (locking unity)
        uinput.write(evdev.ecodes.EV_KEY, evdev.ecodes.KEY_POWER, 1)
        uinput.write(evdev.ecodes.EV_KEY, evdev.ecodes.KEY_POWER, 0)
        uinput.syn()

    def unlock_greeter(self):
        """
        Unlock the greeter to display home screen
        """
        self.main_window.get_greeter().swipe()

    def validate_response(self, response, expected_status_code=200):
        """
        Validate the received response status code against expected code
        :param response: response to validate
        :param expected_status_code: value of expected http status code
        """
        self.assertThat(response.status, Equals(expected_status_code))

    def _assert_notification_dialog(self, notification, summary=None,
                                    body=None, icon=True, secondary_icon=False,
                                    opacity=None):
        """
        Assert that the properties of the notification are as
        expected
        :param notification: notification object to validate
        :param summary: expected notification summary value
        :param body: expected notification body value
        :param icon: expected icon status
        :param secondary_icon: expected secondary icon status
        :param opacity: expected opacity value
        """
        if summary is not None:
            self.assertThat(notification.summary, Eventually(Equals(summary)))
        if body is not None:
            self.assertThat(notification.body, Eventually(Equals(body)))
        if opacity is not None:
            self.assertThat(notification.opacity, Eventually(Equals(opacity)))

        if icon:
            self.assertThat(
                notification.iconSource, Eventually(NotEquals('')))
        else:
            self.assertThat(
                notification.iconSource, Eventually(Equals('')))

        if secondary_icon:
            self.assertThat(
                notification.secondaryIconSource, Eventually(NotEquals('')))
        else:
            self.assertThat(
                notification.secondaryIconSource, Eventually(Equals('')))

    def validate_notification_not_displayed(self, wait=True):
        """
        Validate that the notification is not displayed
        If wait is True then wait for default timeout period
        If wait is False then do not wait at all
        :param wait: wait status
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

    def send_push_broadcast_message(self):
        """
        Send a push broadcast message which should trigger a notification
        to be displayed on the client
        """
        # create a copy of the device's build info
        device_info = self.create_device_info_copy()
        # increment the build number to trigger an update
        device_info.inc_build_number()
        # create push message based on the device data
        push_msg = self.push_helper.create_push_message(
            data=device_info.to_json())
        # send the notification message to the server and check response
        response = self.push_helper.send_push_broadcast_notification(
            push_msg.to_json(), self.test_config.server_listener_addr)
        self.validate_response(response)

    def get_notification_dialog(self, wait=True):
        """
        Get the notification dialog being displaye on screen
        If wait is True then wait for default timeout period
        If wait is False then do not wait at all
        :param wait: wait status
        :return: dialog introspection object
        """
        if wait is True:
            dialog = self.main_window.wait_select_single(
                'Notification', objectName='notification1')
        else:
            dialog = self.main_window.select_single(
                'Notification', objectName='notification1')
        return dialog

    def validate_and_dismiss_notification_dialog(self, message):
        """
        Validate a notification dialog is displayed and dismiss it
        :param message: expected message displayed in summary
        """
        # get the dialog
        dialog = self.get_notification_dialog()
        # validate dialog
        self._assert_notification_dialog(
            dialog, summary=message)
        # press dialog to dismiss
        self.press_notification_dialog(dialog)
        # check the dialog is no longer displayed
        self.validate_notification_not_displayed(wait=False)

    def press_notification_dialog(self, dialog):
        """
        Press the dialog to dismiss it
        """
        self.touch.tap_object(dialog)

    def dismiss_outstanding_dialog(self):
        """
        Dismiss outstanding notification dialog that may be displayed
        from an aborted previous test
        """
        try:
            dialog = self.main_window.select_single(
                'Notification', objectName='notification1')
        except dbus.StateNotFoundError:
            dialog = None
        if dialog:
            self.press_notification_dialog(dialog)
