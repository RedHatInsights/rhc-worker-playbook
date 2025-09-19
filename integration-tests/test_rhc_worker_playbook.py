import logging
import os
import subprocess
import json
import time

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
    http_server,
    rhc_worker_test_file,
    rhc_worker_playbook_config_for_worker_test,
    yggdrasil_config_for_local_mqtt_broker,
):
    """
    test_steps:
            1. Start rhc_worker_playbook
            2. Start yggdrasil service
            3. Verify the test file does not exist
            4. Build a MQTT message to run the playbook
            5. Publish the message to the MQTT topic
            6. Verify the playbook execution status
            7. Verify the test file is created
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
    
    # TODO: re-add this test when the other PR is merged
    # logger.info("Verifying job event data is being correctly filtered......")
    
    # assert verify_uploaded_event_runner_data_is_filtered(http_server.post_body)

@pytest.mark.skipif(
    pytest.rhel_major_version == "unknown" or int(pytest.rhel_major_version) < 10,
    reason="This test is only supported on RHEL10 and above",
)
def test_playbook_execution_timeout_greater_than_one_min(
    http_server,
    rhc_worker_playbook_config_for_worker_test,
    yggdrasil_config_for_local_mqtt_broker,
):
    """
        test_steps:
            1. Start rhc_worker_playbook
            2. Start yggdrasil service
            3. Build a MQTT message to run the playbook
            4. Publish the message to the MQTT topic
            5. Verify the playbook execution status
            6. Verify that the uploads do not continue after execution completes
        expected_results:
            1. The playbook execution is successful
            2. The uploads do not continue indefinitely once execution completes
    """
    playbook_url = "http://localhost:8000/resources/pause1m.yml"
    repsonse_interval = 30

    subprocess.run(
        ["systemctl", "restart", "com.redhat.Yggdrasil1.Worker1.rhc_worker_playbook"]
    )
    subprocess.run(["systemctl", "restart", "yggdrasil"])

    logger.info(f"Playbook will be downloaded from: {playbook_url}")
    data_message = build_data_msg_for_worker_playbook(response_interval=repsonse_interval, content=playbook_url)
    topic = mqtt_data_topic()

    logger.info(f"Publishing message to MQTT broker. Topic: {topic}")
    publish_message(topic=topic, payload=json.dumps(data_message))

    logger.info("Verifying playbook execution status......")

    # playbook takes 1 min to run, allow for 1 min + a little extra
    assert verify_playbook_execution_status(
        data_message["metadata"]["crc_dispatcher_correlation_id"], timeout=90
    )

    logger.info("Verifying the playbook doesn't upload anything more after finishing...")
    # wait just over 30 seconds (response interval) after the playbook finishes to make sure no more uploads come in 
    number_of_uploads_at_playbook_finish = len(http_server.request_bodies)
    start_time = time.time()
    while (time.time() - start_time) < repsonse_interval + 5:
        if len(http_server.request_bodies) > number_of_uploads_at_playbook_finish:
            # something came in after upload finished, this is a failure case
            assert False
        time.sleep(5)
    # number of uploads should be unchanged from before the wait
    assert number_of_uploads_at_playbook_finish == len(http_server.request_bodies)
