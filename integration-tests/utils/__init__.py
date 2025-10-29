import contextlib
import sys
import time
import uuid
import json
import logging
from datetime import datetime

import paho.mqtt.client as mqtt

logger = logging.getLogger(__name__)

def build_data_msg_for_worker_playbook(
    response_interval=600,
    content="http://localhost:8000/cq:qreate_file.yml",
    **data
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


def verify_playbook_execution_status(crc_dispatcher_correlation_id, timeout=30):
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
    return False

def verify_uploaded_event_runner_data_is_filtered(req_body):
    """
    Check that the payload sent by rhc-worker-playbook is filtered down
    to only the required data.
    """
    json_lines = req_body.split('\n')
    for line in json_lines:
        if line == '':
            # request can end with newline and split results in empty string
            continue
        
        parsed = json.loads(line)
        
        # just use current known PBD schema to validate
        for prop in parsed:
            if prop not in ['event', 'uuid', 'counter', 'stdout', 'start_line', 'end_line', 'event_data']:
                return False
        
        for prop in parsed.get('event_data', {}):
            if prop not in ['playbook', 'playbook_uuid', 'host', 'crc_dispatcher_correlation_id', 'crc_dispatcher_error_code', 'crc_dispatcher_error_details']:
                return False

    return True
