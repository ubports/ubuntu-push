# -*- Mode: Python; coding: utf-8; indent-tabs-mode: nil; tab-width: 4 -*-
# Copyright 2014 Canonical
#
# This program is free software: you can redistribute it and/or modify it
# under the terms of the GNU General Public License version 3, as published
# by the Free Software Foundation.


import os

CONFIG_FILE = 'push.conf'

def get_config_file():
    config_dir = os.path.dirname(__file__)
    return os.path.join(config_dir, CONFIG_FILE)

