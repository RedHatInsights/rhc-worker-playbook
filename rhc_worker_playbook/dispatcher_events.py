'''
Event-generating functions for the Playbook Dispatcher
'''

# required event for cloud connector
def executor_on_start(uuid=None, correlation_id=None, stdout=None):
    return {
        "event": "executor_on_start",
        "uuid": uuid,
        "counter": -1,
        "stdout": stdout,
        "start_line": 0,
        "end_line": 0,
        "event_data": {
            "crc_dispatcher_correlation_id": correlation_id
        }
    }

# required event for cloud connector
def executor_on_failed(uuid=None, correlation_id=None, stdout=None, error_code=None, error_details=None):
    return {
        "event": "executor_on_failed",
        "uuid": uuid,
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