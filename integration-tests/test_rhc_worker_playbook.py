import logging
import os
import subprocess
import json

import pytest

from utils import (
    build_data_msg_for_worker_playbook,
    publish_message,
    mqtt_data_topic,
    verify_playbook_execution_status,
)

logger = logging.getLogger(__name__)


@pytest.mark.skipif(
    pytest.rhel_major_version == "unknown" or int(pytest.rhel_major_version) < 10,
    reason="This test is only supported on RHEL10 and above",
)
def test_playbook_execution_local_broker(
    start_http_server_localhost,
    rhc_worker_test_file,
    rhc_worker_playbook_config_for_worker_test,
    yggdrasil_config_for_local_mqtt_broker,
):
    """
    test_steps:
            1. Start rhc_worker_playbook
            2. Start yggdrasil service
            3. Verify the test file does not exist
            3. Build a MQTT message to run the playbook
            4. Publish the message to the MQTT topic
            5. Verify the playbook execution status
            6. Verify the test file is created
        expected_results:
            1. The playbook execution is successful
            2. The test file is created
    """
    playbook_url = "http://localhost:8000/resources/create_file.yml"

    subprocess.run(
        ["systemctl", "restart", "com.redhat.Yggdrasil1.Worker1.rhc_worker_playbook"]
    )
    subprocess.run(["systemctl", "restart", "yggdrasil"])

    assert not os.path.exists(
        rhc_worker_test_file
    ), "Test file already exists when it shouldn't"

    logger.info(f"Playbook will be downloaded from: {playbook_url}")
    data_message = build_data_msg_for_worker_playbook(content=playbook_url)
    topic = mqtt_data_topic()

    logger.info(f"Publishing message to MQTT broker. Topic: {topic}")
    publish_message(topic=topic, payload=json.dumps(data_message))

    logger.info("Verifying playbook execution status......")

    assert verify_playbook_execution_status(
        data_message["metadata"]["crc_dispatcher_correlation_id"]
    )
    assert os.path.exists(rhc_worker_test_file), "Test file not created."
