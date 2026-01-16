import contextlib
import subprocess
import sys
import time
import uuid
import json
import logging
from datetime import datetime

import paho.mqtt.client as mqtt

logger = logging.getLogger(__name__)


def build_data_msg_for_worker_playbook(
    response_interval=600, content="http://localhost:8000/create_file.yml", **data
):
    """
    Provides the mqtt data message in a format accepted by rhc-worker-playbook directive
    """
    return {
        "type": "data",
        "message_id": str(uuid.uuid4()),
        "version": 1,
        "sent": datetime.now().astimezone().replace(microsecond=0).isoformat(),
        "directive": "rhc_worker_playbook",
        "metadata": {
            "response_interval": str(response_interval),
            "crc_dispatcher_correlation_id": str(uuid.uuid4()),
            "return_url": "http://localhost:8000/",
        },
        "content": content,
        **data,
    }


def publish_message(
    host="localhost", port=1883, keepalive=60, topic=None, payload=None
):
    """Function to publish mqtt message to given mqtt topic"""
    client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2)

    if client.connect(host=host, port=port, keepalive=keepalive) != 0:
        print("Couldn't connect to the local mqtt broker")
        sys.exit(1)
    client.publish(topic, payload, qos=1)
    client.disconnect()


def mqtt_data_topic():
    """Return a data topic where yggdrasil workers can publish and subscribe for data messages"""

    return f"test-yggdrasil/{get_yggdrasil_client_id()}/data/in"


def get_yggdrasil_client_id():
    """
    Returns the value of yggdrasil client id
    """
    with open("/var/lib/yggdrasil/client-id", "r") as f:
        yggdrasil_client_id = f.read().strip()
    return yggdrasil_client_id


def verify_playbook_execution_status(crc_dispatcher_correlation_id, timeout=60):
    """
    This method returns True if the playbook execution succeeds else False
    """
    execution_status_file_path = f"/var/lib/rhc-worker-playbook/runs/artifacts/{crc_dispatcher_correlation_id}/status"
    start_time = time.time()
    while (time.time() - start_time) < timeout:
        with contextlib.suppress(FileNotFoundError):
            with open(execution_status_file_path) as f:
                if "successful" in f.read():
                    return True
        time.sleep(5)
        print(f"Waiting for playbook execution status file to be created. Time elapsed: {time.time() - start_time} seconds")
        print(f"Execution status file path: {execution_status_file_path}")
        print(f"Execution status file contents: {open(execution_status_file_path).read()}") #TODO: remove this after debugging
    return False


def verify_uploaded_event_runner_data_is_filtered(req_body):
    """
    Check that the payload sent by rhc-worker-playbook is filtered down
    to only the required data.
    """
    json_lines = req_body.split("\n")
    for line in json_lines:
        if line == "":
            # request can end with newline and split results in empty string
            continue

        parsed = json.loads(line)

        # just use current known PBD schema to validate
        for prop in parsed:
            if prop not in [
                "event",
                "uuid",
                "counter",
                "stdout",
                "start_line",
                "end_line",
                "event_data",
            ]:
                return False

        for prop in parsed.get("event_data", {}):
            if prop not in [
                "playbook",
                "playbook_uuid",
                "host",
                "crc_dispatcher_correlation_id",
                "crc_dispatcher_error_code",
                "crc_dispatcher_error_details",
            ]:
                return False

    return True


def verify_playbook_verification_success_log(timeout=30):
    """
    This method returns True if the playbook verifies successfully, else False
    """
    start_time = time.time()
    while (time.time() - start_time) < timeout:
        if _is_in_journald_grep("Playbook verified."):
            return True
        time.sleep(5)
    return False


def verify_playbook_verification_failure_log(timeout=30):
    """
    This method returns True if the failure of playbook verification
    is logged as an error, else False
    """
    start_time = time.time()
    while (time.time() - start_time) < timeout:
        if _is_in_journald_grep("cannot verify playbook"):
            return True
        time.sleep(5)
    return False


def verify_playbook_verification_failure_upload(http_server, timeout=30):
    """
    This method returns True if the failure of playbook verification
    is transmitted as an event to the HTTP server, else False
    """
    start_time = time.time()
    while (time.time() - start_time) < timeout:
        if http_server.post_body:
            json_obj = http_server.post_body.strip()
            logger.info(http_server.post_body)
            parsed = json.loads(json_obj)

            # there should be 2 events - start and failed
            if len(parsed) != 2:
                return False

            # the first event should be a start event
            event_1 = parsed[0]
            if not (event_1.get("event") == "executor_on_start"):
                return False

            # the second event should be a failure event
            event_2 = parsed[1]
            if not (
                event_2.get("event") == "executor_on_failed"
                and event_2.get("event_data", {}).get("crc_dispatcher_error_code")
                == "ANSIBLE_PLAYBOOK_SIGNATURE_VALIDATION_FAILED"
                and "cannot verify playbook"
                in event_2.get("event_data", {}).get("crc_dispatcher_error_details")
            ):
                return False

            # all conditions are met
            return True
        time.sleep(5)
    return False


def verify_playbook_failure_upload_failed(timeout=30):
    """
    This method returns True if, when the playbook failure event fails to upload,
    a message is logged to console, else False
    """
    start_time = time.time()
    while (time.time() - start_time) < timeout:
        if _is_in_journald_grep("cannot transmit events"):
            return True
        time.sleep(5)
    return False


def _is_in_journald_grep(search_string):
    """
    Check journald logs for rhc-worker-playbook for the presence of
    search_string.
    """
    try:
        journald_grep = subprocess.check_output(
            [
                "journalctl",
                "-Iu",
                "com.redhat.Yggdrasil1.Worker1.rhc_worker_playbook",
                "-g",
                search_string,
                "-o",
                "cat",
            ],
            text=True,
        )
        return search_string in journald_grep
    except subprocess.CalledProcessError:
        pass
