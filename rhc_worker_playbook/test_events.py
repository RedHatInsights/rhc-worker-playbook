import unittest
from typing import ClassVar
from rhc_worker_playbook.events import Events


class TestFilterEvent(unittest.TestCase):

    events: ClassVar[Events]

    @classmethod
    def setUpClass(cls) -> None:
        # just need a single class instance to access filter_event
        cls.events = Events()

    def test_filter_event_default_schema(self) -> None:
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
            "uuid": "93bbe81b-5992-4d70-8cdb-1cd4d9791710",
        }

        # call _filter_event directly for test purposes
        filtered_event = self.events._filter_event(job_event)

        assert filtered_event == {
            "counter": 6,
            "event": "runner_on_start",
            "start_line": 6,
            "end_line": 6,
            "stdout": "",
            "uuid": "93bbe81b-5992-4d70-8cdb-1cd4d9791710",
            "runner_ident": "dcdc7b28-6800-4af9-983a-60fda58a7156",
        }

    def test_filter_event_default_schema_with_recursion(self) -> None:
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
                "uuid": "93bbe81b-5992-4d70-8cdb-1cd4d9791710",
            },
            "parent_uuid": "080027c2-7382-b2cc-1967-000000000008",
            "pid": 4652,
            "runner_ident": "dcdc7b28-6800-4af9-983a-60fda58a7156",
            "start_line": 6,
            "stdout": "",
            "uuid": "93bbe81b-5992-4d70-8cdb-1cd4d9791710",
        }

        # call _filter_event directly for test purposes
        filtered_event = self.events._filter_event(job_event)

        assert filtered_event == {
            "counter": 6,
            "event": "runner_on_start",
            "start_line": 6,
            "end_line": 6,
            "stdout": "",
            "uuid": "93bbe81b-5992-4d70-8cdb-1cd4d9791710",
            "runner_ident": "dcdc7b28-6800-4af9-983a-60fda58a7156",
            "event_data": {
                "crc_dispatcher_correlation_id": "dcdc7b28-6800-4af9-983a-60fda58a7156",
                "host": "localhost",
                "playbook": "/var/lib/rhc-worker-playbook/dcdc7b28-6800-4af9-983a-60fda58a7156.yaml",
                "playbook_uuid": "d0c79d62-4395-41f2-8bc4-8a73ad1df099",
            },
        }

    def test_filter_event_no_matching_keys(self) -> None:
        job_event = {"invalid_key": "invalid_value"}

        filtered_event = self.events._filter_event(job_event)

        assert filtered_event == {}
