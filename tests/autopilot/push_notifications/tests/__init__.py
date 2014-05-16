# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.

"""push-notifications autopilot tests."""


import copy
import time

from unity8.shell.tests import UnityTestCase
import unity8.process_helpers as unity8_helpers
from push_notifications import config as push_config
import push_notifications.helpers.push_notifications_helper as push_helper


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

    def lock_greeter(self):
        """
        Lock the device to display greeter screen
        """
        print('locking delay...')
        time.sleep(10)
        print('Locking greeter')
        #unity8_helpers.lock_unity(self.unity)
        self.press_power_button()
        print('screen should be off')
        time.sleep(5)
        print('turning on')
        self.press_power_button()
        print('screen should be on')
        time.sleep(5)
        print('Greeter should be locked')
        greeter = self.main_window.get_greeter()
        if not greeter.created:
            raise RuntimeWarning('Greeter is not displayed')

    def unlock_greeter(self):
        """
        Unlock the greeter to display home screen
        """
        unity8_helpers.unlock_unity(self.unity)

