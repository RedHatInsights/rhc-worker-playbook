import os
WORKER_LIB_DIR = os.path.join(os.path.dirname(__file__), "contrib")
CONFIG_FILE = os.path.join(os.path.dirname(os.path.dirname(__file__)), "rhc-worker-playbook.toml")
ANSIBLE_COLLECTIONS_PATHS = "/usr/share/rhc-worker-playbook/ansible/collections/ansible_collections/"
# The path to the directory where artifacts should live.
RUNNER_ARTIFACTS_DIR = "/var/log/rhc-worker-playbook/ansible"
# Number of artifact directories we want to keep at most.
RUNNER_ROTATE_ARTIFACTS = 100
