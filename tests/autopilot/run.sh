#!/usr/bin/env bash

usage()
{
cat << EOF
usage: $0 options

OPTIONS:
   -d      adb device_id 
EOF
}

while getopts "d:" opt; do
        case $opt in
                d) DEVICE_ID=$OPTARG;;
                *) usage
                        exit 1
                        ;;
        esac
    done


DEVICE_ID=${DEVICE_ID:-"emulator-5554"}
BASE_DIR="ubuntu-push/src/launchpad.net"
BRANCH_DIR="$BASE_DIR/ubuntu-push"
adb -s ${DEVICE_ID} shell "su - phablet bash -c 'cd ${BRANCH_DIR}/tests/autopilot/; /sbin/initctl stop unity8; autopilot3 run -v push_notifications'"
