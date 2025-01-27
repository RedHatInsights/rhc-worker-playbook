import logging
import os
import shutil
import subprocess

import pytest
import toml

logger = logging.getLogger(__name__)


@pytest.fixture
def start_http_server_localhost():
    """
    Run http server in current directory, it enables download of playbooks.
    """
    command = ["nohup", "/usr/libexec/platform-python", "-m", "http.server", "8000"]
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    logger.info("http server started...")

    yield "http://localhost:8000"
    process.terminate()
    process.wait()


@pytest.fixture
def rhc_worker_test_file():
    """Yield a test file path

    During fixture tear-down, it tries to remove the test file
    """
    test_file = "/tmp/sample-playbook-output.txt"

    yield test_file

    try:
        os.remove(test_file)
    except OSError:
        pass


@pytest.fixture
def rhc_worker_playbook_config_for_worker_test():
    """Setup rhc-worker-playbook configuration for the rhc-worker-playbook test,
    disabling playbook verification for custom written playbooks.

    During fixture tear-down, the default configuration will be restored
    """
    logger.info("Disabling rhc-worker-playbook signature verification...")
    config_path = "/etc/rhc-worker-playbook/rhc-worker-playbook.toml"
    backup_path = "/etc/rhc-worker-playbook/rhc-worker-playbook_backup.toml"
    shutil.copyfile(config_path, backup_path)
    config = toml.load(config_path)
    config["verify-playbook"] = False
    config["insights-core-gpg-check"] = False
    with open(config_path, "w") as configfile:
        toml.dump(config, configfile)

    yield

    logger.info("Restoring rhc-worker-playbook original config...")
    shutil.copyfile(backup_path, config_path)
    os.remove(backup_path)


@pytest.fixture
def yggdrasil_config_for_local_mqtt_broker():
    """Setup yggdrasil config.toml configuration for running tests on local mqtt broker,
    During fixture tear-down, the default configuration will be restored
    """
    logger.info("Disabling rhc-worker-playbook signature verification...")
    config_path = "/etc/yggdrasil/config.toml"
    backup_path = "/etc/yggdrasil/config_backup.toml"
    shutil.copyfile(config_path, backup_path)
    config = toml.load(config_path)
    config["server"] = ["tcp://localhost:1883"]
    config["data-host"] = "localhost:8000"
    config["cert-file"] = ""
    config["key-file"] = ""
    config["facts-file"] = ""

    with open(config_path, "w") as configfile:
        toml.dump(config, configfile)

    yield

    logger.info("Restoring yggdrasil original config...")
    shutil.copyfile(backup_path, config_path)
    os.remove(backup_path)
