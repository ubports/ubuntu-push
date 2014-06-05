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


import os

CONFIG_FILE = 'push.conf'


def get_config_file():
    """
    Return the path for the config file
    """
    config_dir = os.path.dirname(__file__)
    return os.path.join(config_dir, CONFIG_FILE)


def get_cert_file(cert_file_name):
    """
    Return the path for the testing certificate file
    """
    config_dir = os.path.dirname(__file__)
    return os.path.join(config_dir, cert_file_name)
