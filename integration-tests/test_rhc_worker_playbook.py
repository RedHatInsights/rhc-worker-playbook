import logging
import os
import json
import time

import pytest

from utils import (
    build_data_msg_for_worker_playbook,
    publish_message,
    mqtt_data_topic,
    verify_playbook_execution_status,
    verify_playbook_verification_failure_log,
    verify_playbook_verification_failure_upload,
    verify_playbook_verification_success_log,
    verify_uploaded_event_runner_data_is_filtered,
    verify_playbook_failure_upload_failed,
)
from conftest import FakeRequestHandlerPOSTFails

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
    restart_services,
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
            8. Verify that the output is correctly filtered (reduced to minimum playbook dispatcher spec properties)
        expected_results:
            1. The playbook execution is successful
            2. The test file is created
            3. The output is reduced to playbook dispatcher defined properties
    """
    playbook_url = "http://localhost:8000/resources/create_file.yml"

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
    logger.info("Verifying job event data is being correctly filtered......")
    assert verify_uploaded_event_runner_data_is_filtered(http_server.post_body)


@pytest.mark.skipif(
    pytest.rhel_major_version == "unknown" or int(pytest.rhel_major_version) < 10,
    reason="This test is only supported on RHEL10 and above",
)
def test_playbook_execution_timeout_greater_than_one_min(
    http_server,
    rhc_worker_playbook_config_for_worker_test,
    yggdrasil_config_for_local_mqtt_broker,
    restart_services,
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
    response_interval = 30

    logger.info(f"Playbook will be downloaded from: {playbook_url}")
    data_message = build_data_msg_for_worker_playbook(
        response_interval=response_interval, content=playbook_url
    )
    topic = mqtt_data_topic()

    logger.info(f"Publishing message to MQTT broker. Topic: {topic}")
    publish_message(topic=topic, payload=json.dumps(data_message))

    logger.info("Verifying playbook execution status......")

    # playbook takes 1 min to run, allow for 1 min + a little extra
    assert verify_playbook_execution_status(
        data_message["metadata"]["crc_dispatcher_correlation_id"], timeout=90
    )

    logger.info(
        "Verifying the playbook doesn't upload anything more after finishing..."
    )
    # wait just over 30 seconds (response interval) after the playbook finishes to make sure no more uploads come in
    number_of_uploads_at_playbook_finish = len(http_server.request_bodies)
    start_time = time.time()
    while (time.time() - start_time) < response_interval + 5:
        if len(http_server.request_bodies) > number_of_uploads_at_playbook_finish:
            # something came in after upload finished, this is a failure case
            assert False
        time.sleep(5)
    # number of uploads should be unchanged from before the wait
    assert number_of_uploads_at_playbook_finish == len(http_server.request_bodies)


@pytest.mark.parametrize("enable_verify_playbook", [True])
@pytest.mark.skipif(
    pytest.rhel_major_version == "unknown" or int(pytest.rhel_major_version) < 10,
    reason="This test is only supported on RHEL10 and above",
)
def test_playbook_verify_success(
    http_server,
    rhc_worker_playbook_config_for_worker_test,
    yggdrasil_config_for_local_mqtt_broker,
    restart_services,
):
    """
    test_steps:
        1. Build a MQTT message to run the playbook
        2. Publish the message to the MQTT topic
        3. Verify the playbook passes verification
    expected_results:
        1. The playbook verification succeeds
        2. Success message is logged to journald
    """
    # this playbook is signed
    playbook_url = "http://localhost:8000/resources/create_file.yml"

    logger.info(f"Playbook will be downloaded from: {playbook_url}")
    data_message = build_data_msg_for_worker_playbook(content=playbook_url)
    topic = mqtt_data_topic()

    logger.info(f"Publishing message to MQTT broker. Topic: {topic}")
    publish_message(topic=topic, payload=json.dumps(data_message))

    logger.info("Verifying playbook was verified......")
    assert verify_playbook_verification_success_log()


@pytest.mark.parametrize("enable_verify_playbook", [True])
@pytest.mark.skipif(
    pytest.rhel_major_version == "unknown" or int(pytest.rhel_major_version) < 10,
    reason="This test is only supported on RHEL10 and above",
)
def test_playbook_verify_failure(
    http_server,
    rhc_worker_playbook_config_for_worker_test,
    yggdrasil_config_for_local_mqtt_broker,
    restart_services,
):
    """
    test_steps:
        1. Build a MQTT message to run the playbook
        2. Publish the message to the MQTT topic
        3. Verify the playbook fails verification in the system log
        4. Verify that the failure event is uploaded
    expected_results:
        1. The playbook fails verification and the event is logged to both journald and http
    """
    # this playbook is unsigned
    playbook_url = "http://localhost:8000/resources/pause1m.yml"

    logger.info(f"Playbook will be downloaded from: {playbook_url}")
    data_message = build_data_msg_for_worker_playbook(content=playbook_url)
    topic = mqtt_data_topic()

    logger.info(f"Publishing message to MQTT broker. Topic: {topic}")
    publish_message(topic=topic, payload=json.dumps(data_message))

    logger.info("Verifying playbook verification failed......")

    assert verify_playbook_verification_failure_log()
    assert verify_playbook_verification_failure_upload(http_server)


@pytest.mark.parametrize("enable_verify_playbook", [True])
@pytest.mark.parametrize("request_handler", [FakeRequestHandlerPOSTFails])
@pytest.mark.skipif(
    pytest.rhel_major_version == "unknown" or int(pytest.rhel_major_version) < 10,
    reason="This test is only supported on RHEL10 and above",
)
def test_playbook_verify_failure_event_upload_failure(
    http_server,
    rhc_worker_playbook_config_for_worker_test,
    yggdrasil_config_for_local_mqtt_broker,
    restart_services,
):
    """
    test_steps:
        1. Build a MQTT message to run the playbook
        2. Publish the message to the MQTT topic
    expected_results:
        1. The playbook verification fails
        2. Uploading the event fails
        3. Error is logged to journald
    """
    playbook_url = "http://localhost:8000/resources/pause1m.yml"

    logger.info(f"Playbook will be downloaded from: {playbook_url}")
    data_message = build_data_msg_for_worker_playbook(content=playbook_url)
    topic = mqtt_data_topic()

    logger.info(f"Publishing message to MQTT broker. Topic: {topic}")
    publish_message(topic=topic, payload=json.dumps(data_message))

    logger.info("Verifying failure event upload failed......")

    assert verify_playbook_failure_upload_failed()
