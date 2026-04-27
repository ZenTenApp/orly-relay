#!/usr/bin/env bash
## relay-tester must be installed
if ! command -v "relay-tester" &> /dev/null; then
    echo "relay-tester is not installed."
    echo "run this command to install:"
    echo
    echo "./scripts/relaytester-install.sh"
    exit
fi
rm -rf /tmp/orlytest
export ORLY_LOG_LEVEL=trace
export ORLY_LOG_TO_STDOUT=true
export ORLY_LISTEN=127.0.0.1
export ORLY_PORT=3334
export ORLY_IP_WHITELIST=127.0.0
export ORLY_ADMINS=6d9b216ec1dc329ca43c56634e0dba6aaaf3d45ab878bdf4fa910c7117db0bfa,c284f03a874668eded145490e436b87f1a1fc565cf320e7dea93a7e96e3629d7
export ORLY_ACL_MODE=none
export ORLY_DATA_DIR=/tmp/orlytest
go run . &
sleep 5
relay-tester ws://127.0.0.1:3334 nsec12l4072hvvyjpmkyjtdxn48xf8qj299zw60u7ddg58s2aphv3rpjqtg0tvr nsec1syvtjgqauyeezgrev5nqrp36d87apjk87043tgu2usgv8umyy6wq4yl6tu
killall git.smesh.lol/orly
rm -rf /tmp/orlytest
