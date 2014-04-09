# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.

"""push-notifications autopilot tests."""


import configparser

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


class PushNotificationTestBase(AutopilotTestCase):
    """
    Base class for push notification test cases
    """

    PUSH_CLIENT_CONFIG_FILE = '~/.config/ubuntu-push-client/config.json'
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
        # Read the config data
        self.read_config_file()
        # Read the server device address
        server_device_address = self.get_push_server_device_address()
        # write server device address to the client config
        self.write_push_client_server_address(server_device_address)
        # restart the push client
        self.restart_push_client()
        # validate that the initialisation push message is displayed
        self.validate_push_message(self.DEFAULT_DISPLAY_MESSAGE)
        # setup
        super(PushNotificationTestBase, self).setUp()


    def read_config_file(self):
        """
        Read data from config file
        """
        config_file = config.get_config_file()
        self.config = configparser.ConfigParser()
        self.config.read(config_file)
        # read the name of the environment to use (local/remote)
        self.environment = self.config.get(self.SECTION_DEFAULT, self.KEY_ENVIRONMENT)

    def get_push_server_device_address(self):
        """
        Return the server device address from config file
        """
        return self.config.get(self.environment, self.KEY_SERVER_DEVICE_URL)

    def get_push_server_listener_address(self):
        """
        Return the server device address from config file
        """
        return self.config(self.environment, self.KEY_SERVER_LISTENER_URL)

    def restart_push_client(self):
        """
        Restart the push client
        """

    def write_push_client_server_address(self, server_address):
        """
        Write the server details to the push client config
        """

    def send_push_notification(self, server_address, json_data):
        """
        Send the specified push message to the server
        using an HTTP POST command
        """

    def format_json_data(self, push_message):
        """
        Return the json formatted encoding of push_message including:
        channel, data, expire_after
        """

    def validate_push_message(self, display_message, timeout=10):
        """
        Validate that a notification message is displayed on screen
        """


