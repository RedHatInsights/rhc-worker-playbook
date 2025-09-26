import logging
import os
import shutil
import time
import subprocess

import distro
import pytest
import toml

import concurrent.futures

from http.server import HTTPServer, SimpleHTTPRequestHandler
from requests_toolbelt.multipart import decoder

logger = logging.getLogger(__name__)

class FakeServer(HTTPServer):
    post_body = None
    # save the requests so we can reference them when the playbook finishes
    # prior to the fix, rhc-worker-playbook on RHEL 10 would
    #   upload endlessly because of a broken goroutine
    request_bodies = []

class FakeRequestHandler(SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory='./integration-tests', **kwargs)
    
    def do_GET(self):
        super().do_GET()

    def do_POST(self):
        content_length = int(self.headers.get('Content-Length', 0))
        content_type = self.headers.get('Content-Type')
        body = self.rfile.read(content_length)

        decoded_body = decoder.MultipartDecoder(body, content_type).parts[0].text

        self.server.post_body = decoded_body
        self.server.request_bodies.append(decoded_body)
        logger.info(decoded_body)
        self.send_response(201)
        self.end_headers()
        self.wfile.write(b"Accepted")


@pytest.hookimpl(trylast=True)
def pytest_configure(config):
    if distro.id() == "rhel" or distro.id() == "centos":
        pytest.rhel_version = distro.version()
        pytest.rhel_major_version = distro.major_version()
    else:
        pytest.rhel_version = "unknown"
        pytest.rhel_major_version = "unknown"


@pytest.fixture
def http_server():
    """
    Run http server in current directory, it enables download of playbooks.
    """
    logger.info("Starting http server in 5s...")
    time.sleep(5)
    server = FakeServer(("localhost", 8000), FakeRequestHandler)
    executor = concurrent.futures.ThreadPoolExecutor()
    executor.submit(server.serve_forever)
    executor.shutdown(wait=False)

    yield server
    
    server.shutdown()
    server.server_close()


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
    config["log-level"] = "debug"
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
    logger.info("Setting server to local broker in yggdrasil config.toml...")
    config_path = "/etc/yggdrasil/config.toml"
    backup_path = "/etc/yggdrasil/config_backup.toml"

    shutil.copyfile(config_path, backup_path)

    config = toml.load(config_path)
    config["protocol"] = "mqtt"
    config["server"] = ["tcp://localhost:1883"]
    config["data-host"] = "localhost:8000"
    config["cert-file"] = ""
    config["key-file"] = ""
    config["facts-file"] = ""
    config["log-level"] = "trace"
    config["path-prefix"] = "test-yggdrasil"

    with open(config_path, "w") as configfile:
        toml.dump(config, configfile)

    yield

    logger.info("Restoring yggdrasil original config...")
    shutil.copyfile(backup_path, config_path)
    os.remove(backup_path)


def clear_yggdrasil_journal_logs():
    try:
        subprocess.run(["journalctl", "--rotate"], check=True)
        subprocess.run(["journalctl", "--vacuum-time=1s"], check=True)
    except subprocess.CalledProcessError as e:
        print(f"Error cleaning yggdrasil logs : {e}")


def log_journalctl_yggdrasil_logs():
    """Print yggdrasil logs"""
    try:
        logs = subprocess.check_output(
            ["journalctl", "-u", "yggdrasil", "--no-pager"], text=True
        )
        logger.info(logs)
    except subprocess.CalledProcessError as e:
        print(f"failed to fetch yggdrasil logs : {e}")


@pytest.hookimpl(tryfirst=True)
def pytest_runtest_makereport(item, call):
    """Hook to print yggdrasil logs if test fails"""
    if call.when == "call" and call.excinfo is not None:
        print(
            f"Test '{item.name}' Failed. Journalctl for yggdrasil during test is below. "
        )
        log_journalctl_yggdrasil_logs()


@pytest.fixture(autouse=True)
def manage_journal_logs():
    """Fixture to rotate journal logs before each test"""
    clear_yggdrasil_journal_logs()
    yield
