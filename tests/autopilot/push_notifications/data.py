# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.

"""Push-Notifications autopilot data structure classes"""


class PushNotificationMessage:
    """
    Class to hold all the details required for a
    push notification message
    """
    channel = None
    expire_after = None
    data = None

    def __init__(self, channel='system', data='', expire_after=''):
        self.channel = channel
        self.data = data
        self.expire_after = expire_after

    def json(self):
        """
        Return json representation of message
        """
        json_str = '{{"channel":"{0}", "data":{{{1}}}, "expire_on":"{2}"}}'
        return json_str.format(self.channel, self.data, self.expire_after)


class NotificationData:
    """
    Class to represent notification data including
    Device software channel
    Device build number
    Device model
    Device last update
    Data for the notification
    """
    channel = None
    build_number = None
    device = None
    last_update = None
    version = None
    data = None

    @classmethod
    def from_dbus_info(cls, dbus_info=None):
        """
        Create a new object based on dbus_info if provided
        """
        nd = NotificationData()
        if dbus_info is not None:
            nd.device = dbus_info[1]
            nd.channel = dbus_info[2]
            nd.last_update = dbus_info[3]
            nd.build_number = dbus_info[4]['version']
        return nd

    def inc_build_number(self):
        """
        Increment build number
        """
        self.build_number = str(int(self.build_number) + 1)

    def dec_build_number(self):
        """
        Decrement build number
        """
        self.build_number = str(int(self.build_number) - 1)

    def json(self):
        """
        Return json representation of info based:
        "IMAGE-CHANNEL/DEVICE-MODEL": [BUILD-NUMBER, CHANNEL-ALIAS]"
        """
        json_str = '"{0}/{1}": [{2}, "{3}"]'
        return json_str.format(self.channel, self.device, self.build_number,
                               self.data)
