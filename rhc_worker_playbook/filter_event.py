from jsonschema import Draft7Validator, validators

# parsed from https://github.com/RedHatInsights/playbook-dispatcher/blob/master/schema/ansibleRunnerJobEvent.yaml
JOB_EVENT_SCHEMA = {
    "$id": "ansibleRunnerJobEvent",
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "properties": {
        "event": {"type": "string", "minLength": 3, "maxLength": 50},
        "uuid": {"type": "string", "format": "uuid"},
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
                "crc_dispatcher_correlation_id": {"type": "string", "format": "uuid"},
                "crc_dispatcher_error_code": {"type": "string"},
                "crc_dispatcher_error_details": {"type": "string"},
            },
        },
    },
    "required": ["event", "uuid", "counter", "start_line", "end_line"],
}


def extend_with_ansible(validator_class):
    validate_properties = validator_class.VALIDATORS["properties"]

    def _filter_properties(validator, properties, instance, schema):
        # remove properties from the job event that are not present in the schema
        for key in list(instance.keys()):
            if key not in properties:
                instance.pop(key)

        for error in validate_properties(
            validator,
            properties,
            instance,
            schema,
        ):
            yield error

    def _skip_required(validator, properties, instance, schema):
        # validating required fields is the job of playbook dispatcher, so ignore any errors here
        pass

    return validators.extend(
        validator_class,
        {"properties": _filter_properties, "required": _skip_required},
    )


ansible_schema_validator = extend_with_ansible(Draft7Validator)


def filter_event(event) -> dict:
    ansible_schema_validator(JOB_EVENT_SCHEMA).validate(event)
    return event
