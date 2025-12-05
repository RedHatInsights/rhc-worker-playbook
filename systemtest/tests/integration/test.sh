#!/bin/bash
set -ux

# Install required packages
dnf --setopt install_weak_deps=False install -y \
  podman git-core python3-pip python3-pytest rhc-worker-playbook yggdrasil rhc-playbook-verifier

# get to project root
cd $(git rev-parse --show-toplevel)

podman run --rm --detach --publish 1883:1883 --name mosquitto \
  --security-opt  label=disable --volume \
  "./integration-tests/resources/mosquitto_config/mosquitto.conf:/mosquitto/config/mosquitto.conf" \
  docker.io/eclipse-mosquitto
podman ps -a

python3 -m venv venv

. venv/bin/activate

pip install -r integration-tests/requirements.txt

pytest --junit-xml=./junit.xml -v integration-tests  -o  log_cli=true --log-level=DEBUG
retval=$?

if [ -d "$TMT_PLAN_DATA" ]; then
  cp ./junit.xml "$TMT_PLAN_DATA/junit.xml"
fi

exit $retval
