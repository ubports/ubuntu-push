# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.

"""push-notifications autopilot tests."""


import configparser
import http.client as http
import json
import os
import datetime
import subprocess
import copy
import systemimage.config as sys_info
import time

from unity8.shell.tests import UnityTestCase
import unity8.process_helpers as unity8_helpers
from push_notifications.data import PushNotificationMessage
from push_notifications.data import NotificationData
from push_notifications import config as push_config


class PushClientConfig:
    """
    Container class to read and hold all required server config
    """
    KEY_ADDR = 'addr'
    KEY_LISTENER_PORT = 'listener_port'
    KEY_DEVICE_PORT = 'device_port'
    KEY_CONFIG = 'config'

    def __init__(self, config_file_path):
        """
        Open the file and read the config
        """
        parser = configparser.ConfigParser()
        self.read_config(parser, config_file_path)

    def read_config(self, parser, config_file_path):
        """
        Open the file and read the config
        """
        parser.read(config_file_path)
        server_addr = parser[self.KEY_CONFIG][self.KEY_ADDR]
        device_port = parser[self.KEY_CONFIG][self.KEY_DEVICE_PORT]
        listener_port = parser[self.KEY_CONFIG][self.KEY_LISTENER_PORT]
        addr_fmt = '{0}:{1}'
        self.server_listener_addr = addr_fmt.format(server_addr, listener_port)
        self.server_device_addr = addr_fmt.format(server_addr, device_port)


class PushClientController:
    """
    Class used to reconfigure and re-start the push client for testing
    """

    PUSH_CLIENT_DEFAULT_CONFIG_FILE = '/etc/xdg/ubuntu-push-client/config.json'
    PUSH_CLIENT_CONFIG_FILE = '~/.config/ubuntu-push-client/config.json'

    def restart_push_client_using_config(self, client_config=None):
        """
        Restart the push client using the config provided
        If the config is none then revert to default client behaviour
        """
        if client_config is None:
            # just delete the local custom config file
            # client will then just use the original config
            abs_config_file = self._get_abs_local_config_file_path()
            if os.path.exists(abs_config_file):
                os.remove(abs_config_file)
        else:
            # write the config to local config file
            self._write_client_test_config(client_config)

        # Now re-start the client
        self._restart_push_client()

    def _write_client_test_config(self, client_config):
        """
        Write the test server address to client config file
        """
        # read the original push client config file
        with open(self.PUSH_CLIENT_DEFAULT_CONFIG_FILE) as config_file:
            config = json.load(config_file)
        # change server address
        config['addr'] = client_config.server_device_addr
        # write the config json out to the ~.local address
        abs_config_file = self._get_abs_local_config_file_path()
        config_dir = os.path.dirname(abs_config_file)
        if not os.path.exists(config_dir):
            os.makedirs(config_dir)
        with open(abs_config_file, 'w+') as outfile:
            json.dump(config, outfile, indent=4)
            outfile.close()

    def _get_abs_local_config_file_path(self):
        """
        Return absolute path of ~.local config file
        """
        return os.path.expanduser(self.PUSH_CLIENT_CONFIG_FILE)

    def _control_client(self, command):
        """
        start/stop/restart the ubuntu-push-client using initctl
        """
        subprocess.call(['initctl', command, 'ubuntu-push-client'])

    def _stop_push_client(self):
        """
        Stop the push client
        """
        self._control_client('stop')

    def _start_push_client(self):
        """
        Start the push client
        """
        self._control_client('start')

    def _restart_push_client(self):
        """
        Restart the push client
        """
        self._stop_push_client()
        self._start_push_client()


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
        test_config = PushClientConfig(test_config_file)
        push_client_controller = PushClientController()
        push_client_controller.restart_push_client_using_config(test_config)

    @classmethod
    def tearDownClass(cls):
        """
        Executed once after all tests have completed
        Reset the push client to use the device's original config
        """
        push_client_controller = PushClientController()
        push_client_controller.restart_push_client_using_config(None)

    def setUp(self):
        """
        Setup phase executed before each test
        """
        # setup
        super(PushNotificationTestBase, self).setUp()

        # read and store the test config data
        test_config_file = push_config.get_config_file()
        self.test_config = PushClientConfig(test_config_file)
        # get system device and build info
        self.notification_data = self.get_device_info()
        # start unity8
        self.unity = self.launch_unity()

    def _press_power_button(self):
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
        self._press_power_button()
        print('screen should be off')
        time.sleep(5)
        print('turning on')
        self._press_power_button()
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

    def create_notification_data_copy(self):
        """
        Return a copy of the device's notification data
        """
        return copy.deepcopy(self.notification_data)

    def get_device_info(self):
        """
        Discover the device's model and build info
        - device name e.g. mako
        - channel name e.g. ubuntu-touch/trusty-proposed
        - build_number e.g. 101
        Return a NotificationData object containing info
        """
        # channel info needs to be read from file
        parser = configparser.ConfigParser()
        channel_config_file = '/etc/system-image/channel.ini'
        parser.read(channel_config_file)
        channel = parser['service']['channel']
        return NotificationData(
            device=sys_info.config.device,
            channel=channel,
            build_number=sys_info.config.build_number)

    def send_push_broadcast_notification(self, msg_json):
        """
        Send the specified push message to the server broadcast url
        using an HTTP POST command
        """
        headers = {'Content-type': 'application/json'}
        conn = http.HTTPConnection(self.test_config.server_listener_addr)
        conn.request(
            'POST',
            '/broadcast',
            headers=headers,
            body=msg_json)
        return conn.getresponse()

    def create_push_message(self, channel='system', data='', expire_after=''):
        """
        Return a new push msg
        If no expiry time is given, a future date will be assigned
        """
        if expire_after == '':
            expire_after = self.get_future_iso_time()
        return PushNotificationMessage(
            channel=channel,
            data=data,
            expire_after=expire_after)

    def get_past_iso_time(self):
        """
        Return time 1 year in past in ISO format
        """
        return self.get_iso_time(year_offset=-1)

    def get_near_past_iso_time(self):
        """
        Return time 1 minute in past in ISO format
        """
        return self.get_iso_time(min_offset=-1)

    def get_near_future_iso_time(self):
        """
        Return time 1 minute in future in ISO format
        """
        return self.get_iso_time(min_offset=1)

    def get_future_iso_time(self):
        """
        Return time 1 year in future in ISO format
        """
        return self.get_iso_time(year_offset=1)

    def get_current_iso_time(self):
        """
        Return current time in ISO format
        """
        return self.get_iso_time()

    def get_iso_time(self, year_offset=0, month_offset=0, day_offset=0,
                     hour_offset=0, min_offset=0, sec_offset=0,
                     tz_hour_offset=0, tz_min_offset=0):
        """
        Return an ISO8601 format date-time string, including time-zone
        offset: YYYY-MM-DDTHH:MM:SS-HH:MM
        """
        # calulate target time based on current time and format it
        now = datetime.datetime.now()
        target_time = datetime.datetime(
            year=now.year + year_offset,
            month=now.month + month_offset,
            day=now.day + day_offset,
            hour=now.hour + hour_offset,
            minute=now.minute + min_offset,
            second=now.second + sec_offset)
        target_time_fmt = target_time.strftime('%Y-%m-%dT%H:%M:%S')
        # format time zone offset
        tz = datetime.time(
            hour=tz_hour_offset,
            minute=tz_min_offset)
        tz_fmt = tz.strftime('%H:%M')
        # combine target time and time zone offset
        iso_time = '{0}-{1}'.format(target_time_fmt, tz_fmt)
        return iso_time
