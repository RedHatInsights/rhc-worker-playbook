#!/bin/bash
set -ux

# get to project root
cd ../../../
pip install -r integration-tests/requirements.txt
dnf --setopt install_weak_deps=False install -y \
  podman git-core python3-pip python3-pytest

podman run --detach --publish 1883:1883 --name mosquitto --volume "./mosquitto/config/mosquitto.conf:/mosquitto/config/mosquitto.conf" docker.io/eclipse-mosquitto

python3 -m venv venv

. venv/bin/activate

pip install -r integration-tests/requirements.txt

pytest --junit-xml=./junit.xml -v integration-tests
retval=$?

if [ -d "$TMT_PLAN_DATA" ]; then
  cp ./junit.xml "$TMT_PLAN_DATA/junit.xml"
fi

exit $retval