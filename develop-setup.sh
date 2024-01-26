#!/bin/bash

CONTRIB_DIR=$(pwd -P)/rhc_worker_playbook/contrib

mkdir "${CONTRIB_DIR}"
python3 -m pip install --target "${CONTRIB_DIR}" -r test-requirements.txt
