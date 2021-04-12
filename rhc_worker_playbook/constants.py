import os
WORKER_LIB_DIR = "/usr/lib64/rhc-worker-playbook"
CONFIG_FILE = "/etc/rhc/workers/rhc-worker-playbook.toml"
NEWEST_EGG = os.path.join(os.sep, "var", "lib", "insights", "newest.egg")
STABLE_EGG = os.path.join(os.sep, "var", "lib", "insights", "last_stable.egg")
RPM_EGG = os.path.join(os.sep, "etc", "insights-client", "rpm.egg")
