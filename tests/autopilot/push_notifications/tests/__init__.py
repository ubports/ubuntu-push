# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.

"""push-notifications autopilot tests."""


import copy

from unity8.shell.tests import UnityTestCase
import unity8.process_helpers as unity8_helpers
from push_notifications import config as push_config
import push_notifications.helpers.push_notifications_helper as push_helper
from testtools.matchers import Equals
from autopilot.introspection import dbus


class PushNotificationTestBase(UnityTestCase):
    """
    Base class for push notification test cases
    """
    DEFAULT_DISPLAY_MESSAGE = 'There\'s an updated system image.'

    @classmethod
    def setUpClass(cls):
        """
        Executed once before all the tests run
        Restart the push client using the test config
        """
        test_config_file = push_config.get_config_file()
        test_config = push_helper.PushClientConfig(test_config_file)
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
        test_config_file = push_config.get_config_file()
        self.test_config = push_helper.PushClientConfig(test_config_file)
        # create a push helper object which will do all the message sending
        self.push_helper = push_helper.PushNotificationHelper()
        # get and store device and build info
        self.notification_data = self.push_helper.get_device_info()
        # start unity8
        self.unity = self.launch_unity()
        # dismiss any outstanding dialog
        self.dismiss_outstanding_dialog()

    def create_notification_data_copy(self):
        """
        Return a copy of the device's notification data
        """
        return copy.deepcopy(self.notification_data)

    def press_power_button(self):
        import evdev
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
        unity8_helpers.unlock_unity(self.unity)

    def validate_response(self, response, expected_status_code=200):
        """
        Validate the received response status code against expected code
        """
        self.assertThat(response.status, Equals(expected_status_code))

    def validate_notification_displayed(self,
                                        msg_text=DEFAULT_DISPLAY_MESSAGE):
        """
        Validate that the notification is displayed
        Return the dialog object
        """
        dialog = self.main_window.wait_select_single(
            'Notification', objectName='notification1')
        self.assertEqual(msg_text, dialog.summary)
        return dialog

    def validate_notification_not_displayed(self, wait=True):
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

    def send_valid_push_message(self):
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
        self.validate_response(response)

    def validate_and_dismiss_notification_dialog(self, message):
        """
        Validate a notification dialog is displayed and dismiss it
        """
        # validate dialog
        dialog = self.validate_notification_displayed(message)
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
        if dialog is not None:
            self.press_notification_dialog(dialog)
