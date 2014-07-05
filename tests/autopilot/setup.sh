#!/usr/bin/env bash

usage()
{
cat << EOF
usage: $0 -H <IP> [-b <branch url>]

OPTIONS:
   -H      host/IP where the push server is running.
   -b      tests branch url 
   -d      adb device id
EOF
}

while getopts "H:b:c:" opt; do
        case $opt in
                H) PUSH_SERVER=$OPTARG;;
                b) BRANCH_URL=$OPTARG;;
                c) DEVICE_ID=$OPTARG;; 
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


# in case apt fails to fetch some packages
DEPS_OK=$(adb -s ${DEVICE_ID} shell "[ -f autopilot-deps.ok ] && echo 1 || echo 0")
if [[ ${DEPS_OK} == 1 ]] 
then
    adb -s ${DEVICE_ID} shell "DEBIAN_FRONTEND=noninteractive apt-get -y -qq update"
    # required for running the autopilot tests
    adb -s ${DEVICE_ID} shell "DEBIAN_FRONTEND=noninteractive apt-get -y -qq install unity8-autopilot unity-scope-click bzr"
    adb -s ${DEVICE_ID} shell "touch autopilot-deps.ok"
fi
# fetch the code
BASE_DIR="ubuntu-push/src/launchpad.net"
BRANCH_DIR="$BASE_DIR/ubuntu-push"
BRANCH_OK=$(adb -s ${DEVICE_ID} shell "su - phablet bash -c '[ -d "${BRANCH_DIR}" ] && echo 1 || echo 0'")
if [[ ${BRANCH_OK} == 1 ]] 
then
    adb -s ${DEVICE_ID} shell "su - phablet bash -c 'mkdir -p ${BASE_DIR}'"
    adb -s ${DEVICE_ID} shell "su - phablet bash -c '[ ! -d "${BRANCH_DIR}" ] && bzr branch ${BRANCH} ${BRANCH_DIR}'"
fi
adb -s ${DEVICE_ID} shell "su - phablet bash -c 'sed -i 's/192.168.1.3/${PUSH_SERVER}/' ${BRANCH_DIR}/tests/autopilot/push_notifications/config/push.conf'"

# change the local/dev server config, listen in all interfaces
sed -i 's/127.0.0.1/0.0.0.0/g' ${ROOT_DIR}/sampleconfigs/dev.json
# and start it
cd ${ROOT_DIR}; make run-server-dev 
