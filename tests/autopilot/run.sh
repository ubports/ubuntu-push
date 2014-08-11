#!/usr/bin/env bash

set -e 
set -u

usage() {
    cat << EOF
usage: $0 options

OPTIONS:
   -t      fqn of the tests to run (module, class or method)
   -d      adb device_id 
   -v      run tests in verbose mode
EOF
}

while getopts "t:d:v" opt; do
    case $opt in
        d) DEVICE_ID=$OPTARG;;
        v) VERBOSITY="-v";;
        t) TESTS=$OPTARG;;
        *) usage
            exit 1
            ;;
    esac
done


VERBOSITY=${VERBOSITY:-""}
TESTS=${TESTS:-"push_notifications"}
DEVICE_ID=${DEVICE_ID:-"emulator-5554"}
BASE_DIR="ubuntu-push/src/launchpad.net"
BRANCH_DIR="$BASE_DIR/ubuntu-push"
adb -s ${DEVICE_ID} shell "su - phablet bash -c 'cd ${BRANCH_DIR}/tests/autopilot/; /sbin/initctl stop unity8; autopilot3 run ${VERBOSITY} ${TESTS}'"
