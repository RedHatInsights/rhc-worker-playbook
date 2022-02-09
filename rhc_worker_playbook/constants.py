import os
WORKER_LIB_DIR = os.path.join(os.path.dirname(__file__), "contrib")
CONFIG_FILE = os.path.join(os.path.dirname(os.path.dirname(__file__)), "rhc-worker-playbook.toml")
ANSIBLE_COLLECTIONS_PATHS = "/usr/share/rhc-worker-playbook/ansible/collections/ansible_collections/"
