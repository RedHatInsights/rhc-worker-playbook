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
    """
    Mock an HTTP server to accept uploads
    """

    post_body = None
    # save the requests so we can reference them when the playbook finishes
    # prior to the fix, rhc-worker-playbook on RHEL 10 would
    #   upload endlessly because of a broken goroutine
    request_bodies = []


class FakeRequestHandler(SimpleHTTPRequestHandler):
    """
    Request handler for the mock HTTP server
    """

    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory="./integration-tests", **kwargs)

    def do_GET(self):
        super().do_GET()

    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", 0))
        content_type = self.headers.get("Content-Type")
        body = self.rfile.read(content_length)

        decoded_body = decoder.MultipartDecoder(body, content_type).parts[0].text

        # save the upload to the FakeServer object so we can assert against it
        self.server.post_body = decoded_body
        self.server.request_bodies.append(decoded_body)
        logger.info(decoded_body)
        self.send_response(201)
        self.end_headers()
        self.wfile.write(b"Accepted")


class FakeRequestHandlerPOSTFails(FakeRequestHandler):
    def do_POST(self):
        self.send_response(500)
        self.end_headers()
        self.wfile.write(b"Error")


@pytest.hookimpl(trylast=True)
def pytest_configure(config):
    if distro.id() == "rhel" or distro.id() == "centos":
        pytest.rhel_version = distro.version()
        pytest.rhel_major_version = distro.major_version()
    else:
        pytest.rhel_version = "unknown"
        pytest.rhel_major_version = "unknown"


@pytest.fixture()
def request_handler():
    """
    Request handler to use with http_server fixture.
    Defined separately so it can be overridden.
    """
    return FakeRequestHandler


@pytest.fixture
def http_server(request_handler):
    """
    Run http server in current directory, it enables download of playbooks.
    """
    logger.info("Starting http server in 5s...")
    time.sleep(5)
    server = FakeServer(("localhost", 8000), request_handler)
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


@pytest.fixture()
def enable_verify_playbook():
    """
    Fixture for setting whether to verify playbooks in the rhc-worker-playbook config.
    Defined separately so it can be overridden.
    """
    return False


@pytest.fixture
def rhc_worker_playbook_config_for_worker_test(enable_verify_playbook):
    """Setup rhc-worker-playbook configuration for the rhc-worker-playbook test,
    disabling playbook verification for custom written playbooks.

    During fixture tear-down, the default configuration will be restored
    """
    logger.info("Disabling rhc-worker-playbook signature verification...")
    config_path = "/etc/rhc-worker-playbook/rhc-worker-playbook.toml"
    backup_path = "/etc/rhc-worker-playbook/rhc-worker-playbook_backup.toml"
    shutil.copyfile(config_path, backup_path)
    config = toml.load(config_path)
    config["verify-playbook"] = enable_verify_playbook
    config["log-level"] = "trace"
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


def clear_journal_logs():
    """Clear journal logs for both services before test execution"""
    try:
        subprocess.run(["journalctl", "--rotate"], check=True)
        subprocess.run(["journalctl", "--vacuum-time=1s"], check=True)
    except subprocess.CalledProcessError as e:
        print(f"Error cleaning journal logs : {e}")


def log_journalctl_yggdrasil_logs():
    """Print yggdrasil logs"""
    try:
        logs = subprocess.check_output(
            ["journalctl", "-u", "yggdrasil", "--no-pager"], text=True
        )
        logger.info(logs)
    except subprocess.CalledProcessError as e:
        print(f"failed to fetch yggdrasil logs : {e}")


def log_journalctl_rhc_worker_logs():
    """Print rhc-worker-playbook logs"""
    try:
        logs = subprocess.check_output(
            [
                "journalctl",
                "-u",
                "com.redhat.Yggdrasil1.Worker1.rhc_worker_playbook",
                "--no-pager",
            ],
            text=True,
        )
        logger.info(logs)
    except subprocess.CalledProcessError as e:
        print(f"failed to fetch rhc-worker-playbook logs : {e}")


def log_all_service_logs():
    """Print logs for both yggdrasil and rhc-worker-playbook services"""
    print("=" * 80)
    print("YGGDRASIL SERVICE LOGS:")
    print("=" * 80)
    log_journalctl_yggdrasil_logs()

    print("=" * 80)
    print("RHC-WORKER-PLAYBOOK SERVICE LOGS:")
    print("=" * 80)
    log_journalctl_rhc_worker_logs()


@pytest.hookimpl(tryfirst=True)
def pytest_runtest_makereport(item, call):
    """Hook to print service logs if test fails"""
    if call.when == "call" and call.excinfo is not None:
        print(f"Test '{item.name}' Failed. Service logs during test are below.")
        log_all_service_logs()


@pytest.fixture(autouse=True)
def manage_journal_logs():
    """Fixture to rotate journal logs before each test"""
    clear_journal_logs()
    yield


@pytest.fixture()
def restart_services():
    """Restart the services for each test"""

    subprocess.run(
        ["systemctl", "restart", "com.redhat.Yggdrasil1.Worker1.rhc_worker_playbook"]
    )
    subprocess.run(["systemctl", "restart", "yggdrasil"])
    yield
