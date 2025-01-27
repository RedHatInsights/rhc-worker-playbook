import subprocess
import sys
import time
import uuid
from datetime import datetime

import paho.mqtt.client as mqtt


def build_rhc_data_message(**data):
    """
    Provides the matt data message in a format accepted by rhc-worker-playbook directive
    """
    return {
        "type": "data",
        "message_id": str(uuid.uuid4()),
        "version": 1,
        "sent": datetime.now().astimezone().replace(microsecond=0).isoformat(),
        "directive": "rhc_worker_playbook",
        "metadata": {
            "response_interval": "600",
            "crc_dispatcher_correlation_id": str(uuid.uuid4()),
            "return_url": "http://localhost:8000/",
        },
        "content": "http://localhost:8000/create_file.yml",
        **data,
    }


def wait_for_workers(worker_name, timeout=60, interval=5):
    """wait for specified time for workers to load"""
    start_time = time.time()
    while time.time() - start_time < timeout:
        try:
            output = subprocess.run(
                ["yggctl", "workers", "list"],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                universal_newlines=True,
            )
            if worker_name in output.stdout:
                return True
        except subprocess.CalledProcessError as e:
            print(e)
            return False
        time.sleep(interval)

    print("Timeout reached, expected worker did not load in time")
    return False


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
    """Return a data topic where rhc client can publish and subscribe for data messages"""

    return f"redhat/insights/{get_yggdrasil_client_id()}/data/in"


def get_yggdrasil_client_id():
    """
    Returns the value of yggdrasil client id
    """
    with open("/var/lib/yggdrasil/client-id", "r") as f:
        yggdrasil_client_id = f.read().strip()
    return yggdrasil_client_id


def verify_playbook_execution_status(crc_dispatcher_correlation_id):
    """
    This method returns True if the playbook execution succeeds else False
    """
    execution_status_file_path = f"/var/lib/rhc-worker-playbook/runs/artifacts/{crc_dispatcher_correlation_id}/status"
    try:
        with open(execution_status_file_path) as f:
            return True if f.read() == "successful" else False
    except FileNotFoundError:
        print("Specified file not found")
