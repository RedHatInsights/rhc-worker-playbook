package ansible

import (
	"reflect"
	"testing"
)

func TestFilterEventDefaultSchema(t *testing.T) {
	defaultSchema := getPlaybookDispatcherSchema("../../data/ansibleRunnerJobEvent.yaml")
	jobEvent := map[string]any{
		"counter":      6,
		"created":      "2025-08-19T17:54:40.246295+00:00",
		"end_line":     6,
		"event":        "runner_on_start",
		"parent_uuid":  "080027c2-7382-b2cc-1967-000000000008",
		"pid":          4652,
		"runner_ident": "dcdc7b28-6800-4af9-983a-60fda58a7156",
		"start_line":   6,
		"stdout":       "",
		"uuid":         "93bbe81b-5992-4d70-8cdb-1cd4d9791710",
	}

	filteredEvent := filterEvent(jobEvent, defaultSchema)

	expected := map[string]any{
		"counter":    6,
		"event":      "runner_on_start",
		"start_line": 6,
		"end_line":   6,
		"stdout":     "",
		"uuid":       "93bbe81b-5992-4d70-8cdb-1cd4d9791710",
	}

	eq := reflect.DeepEqual(expected, filteredEvent)
	if !eq {
		t.Errorf(`Expected: %v\nReceived: %v`, expected, filteredEvent)
	}
}

func TestFilterEventDefaultSchemaWithRecursion(t *testing.T) {
	defaultSchema := getPlaybookDispatcherSchema("../../data/ansibleRunnerJobEvent.yaml")
	jobEvent := map[string]any{
		"counter":  6,
		"created":  "2025-08-19T17:54:40.246295+00:00",
		"end_line": 6,
		"event":    "runner_on_start",
		"event_data": map[string]any{
			"crc_dispatcher_correlation_id": "dcdc7b28-6800-4af9-983a-60fda58a7156",
			"crc_message_version":           1,
			"host":                          "localhost",
			"play":                          "This is a sample playbook",
			"play_pattern":                  "localhost",
			"play_uuid":                     "080027c2-7382-b2cc-1967-000000000001",
			"playbook":                      "/var/lib/rhc-worker-playbook/dcdc7b28-6800-4af9-983a-60fda58a7156.yaml",
			"playbook_uuid":                 "d0c79d62-4395-41f2-8bc4-8a73ad1df099",
			"resolved_action":               "ansible.builtin.gather_facts",
			"task":                          "Gathering Facts",
			"task_action":                   "gather_facts",
			"task_args":                     "",
			"task_path":                     "/var/lib/rhc-worker-playbook/dcdc7b28-6800-4af9-983a-60fda58a7156.yaml:1",
			"task_uuid":                     "080027c2-7382-b2cc-1967-000000000008",
			"uuid":                          "93bbe81b-5992-4d70-8cdb-1cd4d9791710",
		},
		"parent_uuid":  "080027c2-7382-b2cc-1967-000000000008",
		"pid":          4652,
		"runner_ident": "dcdc7b28-6800-4af9-983a-60fda58a7156",
		"start_line":   6,
		"stdout":       "",
		"uuid":         "93bbe81b-5992-4d70-8cdb-1cd4d9791710",
	}

	filteredEvent := filterEvent(jobEvent, defaultSchema)

	expected := map[string]any{
		"counter":    6,
		"event":      "runner_on_start",
		"start_line": 6,
		"end_line":   6,
		"stdout":     "",
		"uuid":       "93bbe81b-5992-4d70-8cdb-1cd4d9791710",
		"event_data": map[string]any{
			"crc_dispatcher_correlation_id": "dcdc7b28-6800-4af9-983a-60fda58a7156",
			"host":                          "localhost",
			"playbook":                      "/var/lib/rhc-worker-playbook/dcdc7b28-6800-4af9-983a-60fda58a7156.yaml",
			"playbook_uuid":                 "d0c79d62-4395-41f2-8bc4-8a73ad1df099",
		},
	}

	eq := reflect.DeepEqual(expected, filteredEvent)
	if !eq {
		t.Errorf(`Expected: %v\nReceived: %v`, expected, filteredEvent)
	}
}

func TestFilterEventNoMatchingKeys(t *testing.T) {
	defaultSchema := getPlaybookDispatcherSchema("../../data/ansibleRunnerJobEvent.yaml")
	jobEvent := map[string]any{
		"invalid_key": "invalid_value",
	}

	filteredEvent := filterEvent(jobEvent, defaultSchema)

	eq := reflect.DeepEqual(map[string]any{}, filteredEvent)
	if !eq {
		t.Errorf(`Expected: %v\nReceived: %v`, map[string]any{}, filteredEvent)
	}

}

func TestFilterEventWithArray(t *testing.T) {
	testSchema := map[string]any{
		"properties": map[string]any{
			"array_of_objects": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"item_prop": map[string]any{
							"type": "string",
						},
					},
				},
			},
			"array_of_integer": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "integer",
				},
			},
			"array_of_string": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
	}

	jobEvent := map[string]any{
		"array_of_objects": []map[string]any{
			{
				"item_prop":               "test1",
				"item_prop_not_in_schema": 123,
			},
			{
				"item_prop":               "test2",
				"item_prop_not_in_schema": 456,
			},
			{
				"item_prop": "test3",
			},
		},
		"array_of_integer": []any{1, 2, 3},

		// this property does not match the spec, so it will be omitted in the filtered output
		"array_of_string": "123",
	}

	filteredEvent := filterEvent(jobEvent, testSchema)

	expected := map[string]any{
		"array_of_objects": []map[string]any{
			{"item_prop": "test1"},
			{"item_prop": "test2"},
			{"item_prop": "test3"},
		},
		"array_of_integer": []any{1, 2, 3},
	}

	eq := reflect.DeepEqual(expected, filteredEvent)
	if !eq {
		t.Errorf(`Expected: %v\nReceived: %v`, expected, filteredEvent)
	}
}
