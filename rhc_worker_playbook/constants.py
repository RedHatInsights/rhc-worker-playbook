import os
WORKER_LIB_DIR = os.path.join(os.path.dirname(__file__), "contrib")
CONFIG_FILE = os.path.join(os.path.dirname(os.path.dirname(__file__)), "rhc-worker-playbook.toml")
NEWEST_EGG = os.path.join(os.sep, "var", "lib", "insights", "newest.egg")
STABLE_EGG = os.path.join(os.sep, "var", "lib", "insights", "last_stable.egg")
RPM_EGG = os.path.join(os.sep, "etc", "insights-client", "rpm.egg")