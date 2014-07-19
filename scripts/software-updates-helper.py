#!/usr/bin/python3
# Software Updates Push Notifications helper.
#
# This helper is called with one of two things:
# * regular push messages about updated click packages
# * broadcast messages about system updates
#
# the latter is unique to this helper. Figuring out which of those two
# it is is also this helper's job.

import json
import sys
import time

if len(sys.argv) != 3:
    print("File in and out expected via argv", file=sys.stderr)
    sys.exit(1)

f1, f2 = sys.argv[1:3]

# XXX assuming it's an actionable broadcast. Smarts go here.

obj = {
    "notification": {
        "emblem-counter": {
            "count": 1,
            "visible": True,
        },
        "vibrate": {
            "pattern": [50,150],
            "repeat": 3,
        },
        "card": {
            "summary": "There's an updated system image.",
            "body": "Tap to open the system updater.",
            "actions": ["settings:///system/system-update"],
            "icon": "/usr/share/ubuntu/settings/system/icons/settings-system-update.svg",
            "timestamp": int(time.time()),
            "persist": True,
            "popup": True,
        },
    },
}

json.dump(obj, open(f2,"w"))
