'''
Event-generating functions for the Playbook Dispatcher
'''
import uuid


# required event for cloud connector
def executor_on_start(correlation_id=None, stdout=None):
    return {
        "event": "executor_on_start",
        "uuid": str(uuid.uuid4()),
        "counter": -1,
        "stdout": stdout,
        "start_line": 0,
        "end_line": 0,
        "event_data": {
            "crc_dispatcher_correlation_id": correlation_id
        }
    }


# required event for cloud connector
def executor_on_failed(correlation_id=None, stdout=None, error_code=None, error_details=None):
    return {
        "event": "executor_on_failed",
        "uuid": str(uuid.uuid4()),
        "counter": -1,
        "stdout": stdout,
        "start_line": 0,
        "end_line": 0,
        "event_data": {
            "crc_dispatcher_correlation_id": correlation_id,
            "crc_dispatcher_error_code": error_code,
            "crc_dispatcher_error_details": error_details
        }
    }
