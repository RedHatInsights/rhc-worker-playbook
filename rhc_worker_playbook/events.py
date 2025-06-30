import copy
from typing import Mapping, Type
from jsonschema import Draft7Validator, validators
from rhc_worker_playbook.log import log


def create_ansible_event_validator() -> Draft7Validator:
    """
    Creates an instance of a custom jsonschema validator
    for removing keys absent in the playbook dispatcher schema
    from an Ansible runner job event.

    Based on a snippet from
    https://python-jsonschema.readthedocs.io/en/stable/faq/#why-doesn-t-my-schema-s-default-property-set-the-default-on-my-instance
    """

    # Ansible job event schema from playbook dispatcher parsed from
    # https://github.com/RedHatInsights/playbook-dispatcher/blob/master/schema/ansibleRunnerJobEvent.yaml
    job_event_schema = {
        "$id": "ansibleRunnerJobEvent",
        "$schema": "http://json-schema.org/draft-07/schema#",
        "type": "object",
        "properties": {
            "event": {"type": "string", "minLength": 3, "maxLength": 50},
            "uuid": {"type": "string", "format": "uuid"},
            # The runner_ident property is not *currently* in the playbook dispatcher
            # schema, but is included here to prepare for future improvements
            # to discoverability of ansible-runner logs on the system.
            # See: https://issues.redhat.com/browse/RHINENG-19062
            "runner_ident": {"type": "string", "format": "uuid"},
            "counter": {"type": "integer"},
            "stdout": {"type": ["string", "null"]},
            "start_line": {"type": "integer", "minimum": 0},
            "end_line": {"type": "integer", "minimum": 0},
            "event_data": {
                "type": "object",
                "properties": {
                    "playbook": {"type": "string", "minLength": 1},
                    "playbook_uuid": {"type": "string", "format": "uuid"},
                    "host": {"type": "string"},
                    "crc_dispatcher_correlation_id": {
                        "type": "string",
                        "format": "uuid",
                    },
                    "crc_dispatcher_error_code": {"type": "string"},
                    "crc_dispatcher_error_details": {"type": "string"},
                },
            },
        },
        "required": ["event", "uuid", "counter", "start_line", "end_line"],
    }

    def _filter_properties(validator, properties, instance, schema):
        """Remove properties from the job event that are not present in the schema."""
        # e.g. {"expected", "extra"} - {"expected"}
        for extra_key in set(instance.keys()) - set(properties.keys()):
            instance.pop(extra_key)

        validate_properties = Draft7Validator.VALIDATORS["properties"]
        for error in validate_properties(validator, properties, instance, schema):
            yield error

    def _skip_required(validator, properties, instance, schema):
        """Playbook dispatcher is reponsible for validating fields, so ignore errors here."""

    # creates a class that then eeds to be instantiated with a schema
    custom_validator_class: Type[Draft7Validator] = validators.extend(
        Draft7Validator,
        {"properties": _filter_properties, "required": _skip_required},
    )

    return custom_validator_class(job_event_schema)


class Events(list):
    """
    Extension of list to receive ansible-runner events
    """

    def __init__(self) -> None:
        self.ansible_event_validator: Draft7Validator = create_ansible_event_validator()

    def _filter_event(self, event: Mapping) -> Mapping:
        """
        Filter the Ansible job event to be uploaded.

        This code runs inside a custom event handler for Ansible runner.
        Any modifications to `event` before this function exits will persist to disk
        once Ansible writes the JSON file to the job_events directory.
        The jsonschema validator modifies data in-place.
        So make a copy, and do NOT modify `event` as-is, otherwise the job event JSON files
        written to disk will be truncated.
        """
        filtered_event = copy.deepcopy(event)
        # filtered_event is modified in place
        self.ansible_event_validator.validate(filtered_event)
        return filtered_event

    def addEvent(self, event: Mapping) -> bool:
        event.get("event_data", {}).get("res", {}).pop("ansible_facts", None)

        # filter the event prior to sending it to RHC
        filtered_event = self._filter_event(event)
        log(str(filtered_event))
        self.append(filtered_event)

        # this method must return True for ansible to save job events to disk
        #   at RUNNER_ARTIFACTS_DIR/{runId}/job_events
        return True
