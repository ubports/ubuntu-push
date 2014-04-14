# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.

"""push-notifications autopilot tests."""


import configparser
import httplib2
import json
import os
import psutil
import datetime
import subprocess

from push_notifications import config
from autopilot.testcase import AutopilotTestCase
from autopilot.matchers import Eventually
from autopilot.platform import model
from testtools.matchers import Equals


class PushNotificationMessage:
    """
    Class to hold all the details required for a 
    push notification message
    """
    channel = ''
    expire_after = ''
    data = ''

    def __init__(self, channel='system', data='', expire_after=''):
        self.channel = channel
        self.data = data
        self.expire_after = expire_after

    def json(self):
        """
        Return json string of message
        """
        json_str = '{{"channel":"{0}", "data":{{{1}}}, "expire_on":"{2}"}}'
        return json_str.format(self.channel, self.data, self.expire_after)


class PushNotificationTestBase(AutopilotTestCase):
    """
    Base class for push notification test cases
    """

    PUSH_CLIENT_DEFAULT_CONFIG_FILE = '/etc/xdg/ubuntu-push-client/config.json'
    PUSH_CLIENT_CONFIG_FILE = '~/.config/ubuntu-push-client/config.json'
    PUSH_SERVER_BROADCAST_URL = '/broadcast'
    DEFAULT_DISPLAY_MESSAGE = 'There\'s an updated system image.'
    PUSH_MIME_TYPE = 'application/json'
    SECTION_DEFAULT = 'default'
    KEY_ENVIRONMENT = 'environment'
    KEY_SERVER_DEVICE_URL = 'push_server_device_url'
    KEY_SERVER_LISTENER_URL = 'push_server_listener_url'

    def setUp(self):
        """
        Start the client running with the correct server config
        """
        # setup
        super(PushNotificationTestBase, self).setUp()
        # Read the config data
        self.read_config_file()
        # Read the server device address
        server_device_address = self.get_push_server_device_address()
        # write server device address to the client config
        self.write_client_test_config(server_device_address)
        # restart the push client
        self.restart_push_client()
        # validate that the initialisation push message is displayed
        self.validate_push_message(self.DEFAULT_DISPLAY_MESSAGE)
        # create http lib
        self.http = httplib2.Http()


    def read_config_file(self):
        """
        Read data from config file
        """
        config_file = config.get_config_file()
        self.configparser = configparser.ConfigParser()
        self.configparser.read(config_file)
        # read the name of the environment to use (local/remote)
        self.environment = self.configparser[self.SECTION_DEFAULT][self.KEY_ENVIRONMENT]

    def get_push_server_device_address(self):
        """
        Return the server device address from config file
        """
        return self.configparser[self.environment][self.KEY_SERVER_DEVICE_URL]

    def get_push_server_listener_address(self):
        """
        Return the server device address from config file
        """
        return self.configparser[self.environment][self.KEY_SERVER_LISTENER_URL]

    def _control_client(self, command):
        """
        start/stop/restart the ubuntu-push-client using initctl
        """        
        subprocess.call(['initctl', command, 'ubuntu-push-client'])
    
    def stop_push_client(self):
        """
        Stop the push client
        """
        self._control_client('stop')

    def start_push_client(self):
        """
        Start the push client
        """
        self._control_client('start')

    def restart_push_client(self):
        """
        Restart the push client
        """
        self.stop_push_client()
        self.start_push_client()
        
    def write_client_test_config(self, server_address):
        """
        Write the test server address to client config file
        """
        # read the original config file
        with open(self.PUSH_CLIENT_DEFAULT_CONFIG_FILE) as config_file:    
            config = json.load(config_file)
        # change server address
        config['addr'] = self.get_push_server_device_address()
        # write the config json out to the ~.local address
        abs_config_file = os.path.expanduser(self.PUSH_CLIENT_CONFIG_FILE)
        config_dir = os.path.dirname(abs_config_file) 
        if not os.path.exists(config_dir):
            os.makedirs(config_dir)
        with open(abs_config_file, 'w+') as outfile:
            json.dump(config, outfile, indent=4)

    def send_push_broadcast_notification(self, msg_json):
        """
        Send the specified push message to the server broadcast url
        using an HTTP POST command
        """
        broadcast_url = self.get_push_server_listener_address() + self.PUSH_SERVER_BROADCAST_URL
        headers = {'Content-type': self.PUSH_MIME_TYPE}
        response = self.http.request(
            broadcast_url, 'POST', headers=headers, body=msg_json)
        return response

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

    def validate_push_message(self, display_message, timeout=10):
        """
        Validate that a notification message is displayed on screen
        """

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
            hour_offset=0, min_offset=0, sec_offset=0, tz_hour_offset=0, 
            tz_min_offset=0):
        """
        Return an ISO8601 format date-time string, including time-zone
        offset: YYYY-MM-DDTHH:MM:SS-HH:MM
        """
        # calulate target time based on current time and format it
        now = datetime.datetime.now()
        target_time = datetime.datetime(
            year=now.year+year_offset, 
            month=now.month+month_offset, 
            day=now.day+day_offset, 
            hour=now.hour+hour_offset, 
            minute=now.minute+min_offset, 
            second=now.second+sec_offset)
        target_time_fmt = target_time.strftime('%Y-%m-%dT%H:%M:%S')
        # format time zone offset
        tz = datetime.time(
            hour=tz_hour_offset,
            minute=tz_min_offset)
        tz_fmt = tz.strftime('%H:%M')
        # combine target time and time zone offset
        iso_time = '{0}-{1}'.format(target_time_fmt, tz_fmt)
        return iso_time


