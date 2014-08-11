#!/usr/bin/env bash

set -e 
set -u

usage() { 
    cat << EOF
usage: $0 -H <IP> [-b <branch url>]

OPTIONS:
-H      host/IP where the push server is running.
-b      tests branch url 
-d      adb device id
-u      run apt-get update in the device before installing dependencies
EOF
}

while getopts "H:b:d:u" opt; do
    case $opt in
        H) PUSH_SERVER=$OPTARG;;
        b) BRANCH_URL=$OPTARG;;
        d) DEVICE_ID=$OPTARG;; 
        u) APT_UPDATE="1";;
        *) usage
            exit 1
            ;;
    esac
done

if [[ -z ${PUSH_SERVER} ]]
then
    usage
    exit 1
fi


DEVICE_ID=${DEVICE_ID:-"emulator-5554"}
BRANCH_URL=${BRANCH_URL:-"lp:ubuntu-push/automatic"}
ROOT_DIR=`bzr root`
APT_UPDATE=${APT_UPDATE:-"0"}

DEPS_OK=$(adb -s ${DEVICE_ID} shell "[ ! -f autopilot-deps.ok ] && echo 1 || echo 0")
# get substring [0] of the returned 1/0 value because we get a trailing ^M
if [[ "${DEPS_OK:0:1}" == "1" ]] 
then
    echo "installing dependencies"
    if [[ "${APT_UPDATE}" == "1" ]]
    then
        # in case apt fails to fetch some packages
        adb -s ${DEVICE_ID} shell "DEBIAN_FRONTEND=noninteractive apt-get -y update"
    fi
    # required for running the autopilot tests
    adb -s ${DEVICE_ID} shell "DEBIAN_FRONTEND=noninteractive apt-get -y install unity8-autopilot unity-scope-click bzr"
    adb -s ${DEVICE_ID} shell "touch autopilot-deps.ok"
fi
# fetch the code
BASE_DIR="/home/phablet/ubuntu-push/src/launchpad.net"
BRANCH_DIR="$BASE_DIR/ubuntu-push"
BRANCH_OK=$(adb -s ${DEVICE_ID} shell "su - phablet bash -c '[ ! -d "${BRANCH_DIR}/tests/autopilot" ] && echo 1 || echo 0'")
if [[ "${BRANCH_OK:0:1}" == 1 ]] 
then
    echo "fetching code."
    adb -s ${DEVICE_ID} shell "su - phablet bash -c 'mkdir -p ${BASE_DIR}'"
    adb -s ${DEVICE_ID} shell "su - phablet bash -c 'bzr branch ${BRANCH_URL} ${BRANCH_DIR}'"
fi
adb -s ${DEVICE_ID} shell "su - phablet bash -c 'sed -i 's/addr =.*/addr = ${PUSH_SERVER}/' ${BRANCH_DIR}/tests/autopilot/push_notifications/config/push.conf'"

# copy the trivial-helper.sh as the heper for the messaging-app (used in the tests)
HELPER_OK=$(adb -s ${DEVICE_ID} shell "[ ! -f /usr/lib/ubuntu-push-client/legacy-helpers/messaging-app ] && echo 1 || echo 0")
if [[ "${HELPER_OK:0:1}" == 1 ]] 
then
    adb -s ${DEVICE_ID} shell "cp ${BRANCH_DIR}/scripts/trivial-helper.sh /usr/lib/ubuntu-push-client/legacy-helpers/messaging-app"
fi

# change the local/dev server config, listen in all interfaces
sed -i 's/127.0.0.1/0.0.0.0/g' ${ROOT_DIR}/sampleconfigs/dev.json
# and start it
cd ${ROOT_DIR}; make run-server-dev 
# remove the trivial helper for the messaging-app
adb -s ${DEVICE_ID} shell "rm /usr/lib/ubuntu-push-client/legacy-helpers/messaging-app"
