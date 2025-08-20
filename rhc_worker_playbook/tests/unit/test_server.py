import sys
import os
import types

# Mock toml, grpc, ansible_runner before importing Events
sys.modules['toml'] = types.ModuleType('toml')
sys.modules['grpc'] = types.ModuleType('grpc')
sys.modules['ansible_runner'] = types.ModuleType('ansible_runner')

# Additional mocks for protocol imports
yggdrasil_pb2_grpc_mock = types.ModuleType('yggdrasil_pb2_grpc')
yggdrasil_pb2_grpc_mock.WorkerServicer = object  # Add WorkerServicer attribute
sys.modules['rhc_worker_playbook.protocol.yggdrasil_pb2_grpc'] = yggdrasil_pb2_grpc_mock
sys.modules['rhc_worker_playbook.protocol.yggdrasil_pb2'] = types.ModuleType('yggdrasil_pb2')

# Mock environment variable and schema file path
os.environ['YGG_SOCKET_ADDR'] = ''

# Mock constants module and DEFAULT_JOB_EVENT_SCHEMA_FILE
mock_constants = types.ModuleType('constants')
mock_constants.WORKER_LIB_DIR = ''
mock_constants.CONFIG_FILE = ''
mock_constants.ANSIBLE_COLLECTIONS_PATHS = ''
mock_constants.RUNNER_ARTIFACTS_DIR = ''
mock_constants.RUNNER_ROTATE_ARTIFACTS = ''
# assuming pytest is run from the root of the project
mock_constants.DEFAULT_JOB_EVENT_SCHEMA_FILE = 'ansibleRunnerJobEvent.yml'
sys.modules['rhc_worker_playbook.constants'] = mock_constants

from rhc_worker_playbook.server import Events

class TestEvents:
    def setup_method(self):
        self.events = Events()

    def teardown_method(self):
        pass

    def test_filter_event_default_schema(self):
        job_event = {
            "counter": 6,
            "created": "2025-08-19T17:54:40.246295+00:00",
            "end_line": 6,
            "event": "runner_on_start",
            "parent_uuid": "080027c2-7382-b2cc-1967-000000000008",
            "pid": 4652,
            "runner_ident": "dcdc7b28-6800-4af9-983a-60fda58a7156",
            "start_line": 6,
            "stdout": "",
            "uuid": "93bbe81b-5992-4d70-8cdb-1cd4d9791710"
        }
        
        # call _filter_event directly for test purposes
        filtered_event = self.events._filter_event(job_event, self.events.eventSchema)
        
        assert filtered_event == {
            "counter": 6,
            "event": "runner_on_start",
            "start_line": 6,
            "end_line": 6,
            "stdout": "",
            "uuid": "93bbe81b-5992-4d70-8cdb-1cd4d9791710"
        }
        
    def test_filter_event_default_schema_with_recursion(self):
        job_event = {
            "counter": 6,
            "created": "2025-08-19T17:54:40.246295+00:00",
            "end_line": 6,
            "event": "runner_on_start",
            "event_data": {
                "crc_dispatcher_correlation_id": "dcdc7b28-6800-4af9-983a-60fda58a7156",
                "crc_message_version": 1,
                "host": "localhost",
                "play": "This is a sample playbook",
                "play_pattern": "localhost",
                "play_uuid": "080027c2-7382-b2cc-1967-000000000001",
                "playbook": "/var/lib/rhc-worker-playbook/dcdc7b28-6800-4af9-983a-60fda58a7156.yaml",
                "playbook_uuid": "d0c79d62-4395-41f2-8bc4-8a73ad1df099",
                "resolved_action": "ansible.builtin.gather_facts",
                "task": "Gathering Facts",
                "task_action": "gather_facts",
                "task_args": "",
                "task_path": "/var/lib/rhc-worker-playbook/dcdc7b28-6800-4af9-983a-60fda58a7156.yaml:1",
                "task_uuid": "080027c2-7382-b2cc-1967-000000000008",
                "uuid": "93bbe81b-5992-4d70-8cdb-1cd4d9791710"
            },
            "parent_uuid": "080027c2-7382-b2cc-1967-000000000008",
            "pid": 4652,
            "runner_ident": "dcdc7b28-6800-4af9-983a-60fda58a7156",
            "start_line": 6,
            "stdout": "",
            "uuid": "93bbe81b-5992-4d70-8cdb-1cd4d9791710"
        }
        
        # call _filter_event directly for test purposes
        filtered_event = self.events._filter_event(job_event, self.events.eventSchema)
        
        assert filtered_event == {
            "counter": 6,
            "event": "runner_on_start",
            "start_line": 6,
            "end_line": 6,
            "stdout": "",
            "uuid": "93bbe81b-5992-4d70-8cdb-1cd4d9791710",
            "event_data": {
                "crc_dispatcher_correlation_id": "dcdc7b28-6800-4af9-983a-60fda58a7156",
                "host": "localhost",
                "playbook": "/var/lib/rhc-worker-playbook/dcdc7b28-6800-4af9-983a-60fda58a7156.yaml",
                "playbook_uuid": "d0c79d62-4395-41f2-8bc4-8a73ad1df099",
            }
        }
        
    def test_filter_event_no_matching_keys(self):
        job_event = {
            "invalid_key": "invalid_value"
        }
        
        filtered_event = self.events._filter_event(job_event, self.events.eventSchema)
        
        assert filtered_event == {}
        
    def test_filter_event_with_array(self):
        test_schema = {
            "properties": {
                "array_of_objects": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "properties": {
                            "item_prop": {
                                "type": "string"
                            }
                        }
                    }
                },
                "array_of_integer": {
                    "type": "array",
                    "items": {
                        "type": "integer"
                    }
                },
                "array_of_string": {
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                }
            }
        }
        
        job_event = {
            "array_of_objects": [
                {"item_prop": "test1", "item_prop_not_in_schema": 123},
                {"item_prop": "test2", "item_prop_not_in_schema": 456},
                {"item_prop": "test3"}
            ],
            "array_of_integer": [1,2,3],
            
            # this property does not match the spec, so it will be omitted in the filtered output
            "array_of_string": "123"
        }
        
        # create new events obj since a custom schema is being used
        events = Events(eventSchema=test_schema)
        
        filtered_event = events._filter_event(job_event, events.eventSchema)
        
        assert filtered_event == {
            "array_of_objects": [
                {"item_prop": "test1"},
                {"item_prop": "test2"},
                {"item_prop": "test3"}
            ],
            "array_of_integer": [1,2,3],
        }