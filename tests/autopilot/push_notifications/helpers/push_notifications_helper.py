# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
#
# Push Notifications Autopilot Test Suite
# Copyright (C) 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.
#

import configparser
import datetime
import http.client as http
import json
import os
import subprocess
import systemimage.config as sys_info

from push_notifications import config as push_config
from push_notifications.data import (
    PushNotificationMessage,
    DeviceNotificationData
)


class PushClientConfig:
    """
    Container class to read and hold all required server config
    - Server listener address
    - Server device address
    - Certificate PEM file path
    """

    @staticmethod
    def read_config(config_file_path):
        """
        Return PushClientConfig object containing all test config parameters
        which have been read from specified config file.
        :param config_file_path: path to required config file
        :return: PushClientConfig object containing all config parameters
        """
        KEY_ADDR = 'addr'
        KEY_LISTENER_PORT = 'listener_port'
        KEY_DEVICE_PORT = 'device_port'
        KEY_CONFIG = 'config'
        KEY_CERT_PEM_FILE = 'cert_pem_file'
        KEY_AUTH_HELPER = 'auth_helper'

        config = PushClientConfig()
        parser = configparser.ConfigParser()
        parser.read(config_file_path)
        server_addr = parser[KEY_CONFIG][KEY_ADDR]
        device_port = parser[KEY_CONFIG][KEY_DEVICE_PORT]
        listener_port = parser[KEY_CONFIG][KEY_LISTENER_PORT]
        auth_helper = parser[KEY_CONFIG][KEY_AUTH_HELPER]
        addr_fmt = '{0}:{1}'
        http_addr_fmt = 'http://{0}:{1}/'
        config.server_listener_addr = addr_fmt.format(
            server_addr, listener_port)
        config.server_device_addr = addr_fmt.format(server_addr, device_port)
        config.server_session_url = http_addr_fmt.format(server_addr, listener_port)
        config.server_registration_url = http_addr_fmt.format(server_addr, listener_port)
        config.cert_pem_file = push_config.get_cert_file(
            parser[KEY_CONFIG][KEY_CERT_PEM_FILE])
        config.auth_helper = auth_helper
        return config


class PushClientController:
    """
    Class used to reconfigure and re-start ubuntu-push-client for testing
    """

    PUSH_CLIENT_DEFAULT_CONFIG_FILE = '/etc/xdg/ubuntu-push-client/config.json'
    PUSH_CLIENT_CONFIG_FILE = '~/.config/ubuntu-push-client/config.json'

    def restart_push_client_using_config(self, client_config=None):
        """
        Restart the push client using the config provided
        If the config is none then revert to default client behaviour
        :param client_config: PushClientConfig object containing
                              required config
        """
        if client_config is None:
            # just delete the local custom config file
            # client will then just use the original config
            abs_config_file = self.get_abs_local_config_file_path()
            if os.path.exists(abs_config_file):
                os.remove(abs_config_file)
        else:
            # write the config to local config file
            self.write_client_test_config(client_config)

        # Now re-start the client
        self.restart_push_client()

    def write_client_test_config(self, client_config):
        """
        Write the test server address and certificate path
        to the client config file
        :param client_config: PushClientConfig object containing
                              required config
        """
        # read the original push client config file
        with open(self.PUSH_CLIENT_DEFAULT_CONFIG_FILE) as config_file:
            config = json.load(config_file)
        # change server address
        config['addr'] = client_config.server_device_addr
        # change session_url
        config['session_url'] = client_config.server_session_url
        # change registration url
        config['registration_url'] = client_config.server_registration_url
        # add certificate file path
        config['cert_pem_file'] = client_config.cert_pem_file
        # change the auth_helper
        config['auth_helper'] = client_config.auth_helper
        # write the config json out to the ~.local address
        # creating the directory if it doesn't already exist
        abs_config_file = self.get_abs_local_config_file_path()
        config_dir = os.path.dirname(abs_config_file)
        if not os.path.exists(config_dir):
            os.makedirs(config_dir)
        with open(abs_config_file, 'w+') as outfile:
            json.dump(config, outfile, indent=4)
            outfile.close()

    def get_abs_local_config_file_path(self):
        """
        Return absolute path of ~.local config file
        """
        return os.path.expanduser(self.PUSH_CLIENT_CONFIG_FILE)

    def control_client(self, command):
        """
        start/stop/restart the ubuntu-push-client using initctl
        """
        subprocess.call(
            ['/sbin/initctl', command, 'ubuntu-push-client'],
            stdout=subprocess.DEVNULL)

    def stop_push_client(self):
        """
        Stop the push client
        """
        self.control_client('stop')

    def start_push_client(self):
        """
        Start the push client
        """
        self.control_client('start')

    def restart_push_client(self):
        """
        Restart the push client
        """
        self.stop_push_client()
        self.start_push_client()


class PushNotificationHelper:
    """
    Utility class to create and send push notification messages
    """

    DEFAULT_BROADCAST_URL = '/broadcast'
    UNICAST_URL = '/notify'

    def get_device_info(self):
        """
        Discover the device's model and build info
        - device name e.g. mako
        - channel name e.g. ubuntu-touch/utopic-proposed
        - build_number e.g. 101
        :return: DeviceNotificationData object containing device info
        """
        # channel info needs to be read from file
        parser = configparser.ConfigParser()
        channel_config_file = '/etc/system-image/channel.ini'
        parser.read(channel_config_file)
        channel = parser['service']['channel']
        return DeviceNotificationData(
            device=sys_info.config.device,
            channel=channel,
            build_number=sys_info.config.build_number)

    def send_push_broadcast_notification(self, msg_json, server_addr,
                                         url=DEFAULT_BROADCAST_URL):
        """
        Send the specified push message to the server broadcast url
        using an HTTP POST command
        :param msg_json: JSON representation of message to send
        :param url: destination server url to send message to
        """
        headers = {'Content-type': 'application/json'}
        conn = http.HTTPConnection(server_addr)
        conn.request(
            'POST',
            url,
            headers=headers,
            body=msg_json)
        return conn.getresponse()

    def create_push_message(self, channel='system', data=None,
                            expire_after=None):
        """
        Return a new push message
        If no expiry time is given, a future date will be assigned
        :param channel: name of the channel
        :param data: data value of the message
        :param expire_after: expiry time for message
        :return: PushNotificationMessage object containing specified parameters
        """
        if expire_after is None:
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
        Return time 5 seconds in past in ISO format
        """
        return self.get_iso_time(sec_offset=-5)

    def get_near_future_iso_time(self):
        """
        Return time 5 seconds in future in ISO format
        """
        return self.get_iso_time(sec_offset=5)

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
        :param year_offset: number of years to offset
        :param month_offset: number of months to offset
        :param day_offset: number of days to offset
        :param hour_offset: number of hours to offset
        :param min_offset: number of minutes to offset
        :param sec_offset: number of seconds to offset
        :param tz_hour_offset: number of hours to offset time zone
        :param tz_min_offset: number of minutes to offset time zone
        :return: string representation of required time in ISO8601 format
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

    def _http_request(self, server_addr, url, method='GET',
                      body=None):
        headers = {'Content-type': 'application/json'}
        conn = http.HTTPConnection(server_addr)
        conn.request(
            method,
            url,
            headers=headers,
            body=body)
        return conn.getresponse()

    def register(self, appid):
        """Register the device/appid with the push server."""
        path = appid.split("_")[0].replace(".", "_2e")
        cmd = ["gdbus", "call", "-e", "-d", "com.ubuntu.PushNotifications",
               "-o", "/com/ubuntu/PushNotifications/%s" % path,
               "-m", "com.ubuntu.PushNotifications.Register", appid]
        output = subprocess.check_output(cmd)
        return output[2:-4].decode("utf-8")

    def send_unicast(self, msg, server_addr, url=UNICAST_URL):
        """Send a unicast notification"""
        return self._http_request(server_addr, url, method='POST',
                                  body=json.dumps(msg))
