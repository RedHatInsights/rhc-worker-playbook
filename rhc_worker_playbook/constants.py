import os
WORKER_LIB_DIR = os.path.join(os.path.dirname(__file__), "contrib")
CONFIG_FILE = os.path.join(os.path.dirname(os.path.dirname(__file__)), "rhc-worker-playbook.toml")
RPM_EGG = "/etc/insights-client/rpm.egg"
STABLE_EGG = "/var/lib/insights/last_stable.egg"
