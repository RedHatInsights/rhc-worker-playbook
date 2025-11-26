package ansible

import (
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

// seed uuid.New function for deterministic tests
var seededUuidString string = "080027c2-7382-b2cc-1967-000000000001"
var seededUuid uuid.UUID = uuid.MustParse(seededUuidString)

func mockUuid() uuid.UUID {
	return seededUuid
}

// test that the raw job event is filtered down
func TestFilterJobEvent(t *testing.T) {
	sampleJobEventData := []byte(`{
       "counter": 4,
       "created": "2025-08-19T17:54:40.240100+00:00",
       "end_line": 4,
       "event": "playbook_on_play_start",
       "event_data": {
           "crc_dispatcher_correlation_id": "dcdc7b28-6800-4af9-983a-60fda58a7156",
           "crc_message_version": 1,
           "name": "This is a sample playbook",
           "pattern": "localhost",
           "play": "This is a sample playbook",
           "play_pattern": "localhost",
           "play_uuid": "080027c2-7382-b2cc-1967-000000000001",
           "playbook": "/var/lib/rhc-worker-playbook/dcdc7b28-6800-4af9-983a-60fda58a7156.yaml",
           "playbook_uuid": "d0c79d62-4395-41f2-8bc4-8a73ad1df099",
           "uuid": "080027c2-7382-b2cc-1967-000000000001"
       },
       "parent_uuid": "d0c79d62-4395-41f2-8bc4-8a73ad1df099",
       "pid": 4652,
       "runner_ident": "dcdc7b28-6800-4af9-983a-60fda58a7156",
       "start_line": 2,
       "stdout": "\r\nPLAY [This is a sample playbook] ***********************************************",
       "uuid": "080027c2-7382-b2cc-1967-000000000001"
	}`)

	filteredJobEventData, err := filterJobEvent(sampleJobEventData)

	// should be filtered (different) since attributes were reduced
	if reflect.DeepEqual(filteredJobEventData, sampleJobEventData) {
		t.Errorf(
			"EXPECTED: %v\nRECEIVED: %v",
			string(sampleJobEventData),
			string(filteredJobEventData),
		)
	}

	// there should be no error
	if err != nil {
		t.Errorf("Received unexpected error value: %v", err)
	}
}

func TestFilterJobEventFails(t *testing.T) {
	// unmarhsaling error caused by unexpected type in job event,
	//  "counter": "4" is string rather than int
	// 	and forces filterJobEvent to return the original input data

	sampleJobEventData := []byte(`{
       "counter": "4",
       "created": "2025-08-19T17:54:40.240100+00:00",
       "end_line": 4,
       "event": "playbook_on_play_start",
       "event_data": {
           "crc_dispatcher_correlation_id": "dcdc7b28-6800-4af9-983a-60fda58a7156",
           "crc_message_version": 1,
           "name": "This is a sample playbook",
           "pattern": "localhost",
           "play": "This is a sample playbook",
           "play_pattern": "localhost",
           "play_uuid": "080027c2-7382-b2cc-1967-000000000001",
           "playbook": "/var/lib/rhc-worker-playbook/dcdc7b28-6800-4af9-983a-60fda58a7156.yaml",
           "playbook_uuid": "d0c79d62-4395-41f2-8bc4-8a73ad1df099",
           "uuid": "080027c2-7382-b2cc-1967-000000000001"
       },
       "parent_uuid": "d0c79d62-4395-41f2-8bc4-8a73ad1df099",
       "pid": 4652,
       "runner_ident": "dcdc7b28-6800-4af9-983a-60fda58a7156",
       "start_line": 2,
       "stdout": "\r\nPLAY [This is a sample playbook] ***********************************************",
       "uuid": "080027c2-7382-b2cc-1967-000000000001"
	}`)

	filteredJobEventData, err := filterJobEvent(sampleJobEventData)

	// should be nil
	if filteredJobEventData != nil {
		t.Errorf(
			"EXPECTED: %v\nRECEIVED: %v",
			nil,
			string(filteredJobEventData),
		)
	}

	// error should be returned
	if err == nil {
		t.Errorf("Received unexpected error value: %v", err)
	}
}

func TestGenerateExecutorOnFailedEvent(t *testing.T) {

	expectedFailureEvent := map[string]any{
		"event":      "executor_on_failed",
		"uuid":       seededUuidString,
		"counter":    -1,
		"start_line": 0,
		"end_line":   0,
		"event_data": map[string]any{
			"crc_dispatcher_correlation_id": "dcdc7b28-6800-4af9-983a-60fda58a7156",
			"crc_dispatcher_error_code":     "TEST_ERROR",
			"crc_dispatcher_error_details":  "playbook run failed",
		},
	}
	receivedFailureEvent := GenerateExecutorOnFailedEvent(
		"dcdc7b28-6800-4af9-983a-60fda58a7156",
		"TEST_ERROR",
		errors.New("playbook run failed"),
		mockUuid,
	)

	if !reflect.DeepEqual(expectedFailureEvent, receivedFailureEvent) {
		t.Errorf(
			"EXPECTED: %v\nRECEIVED: %v",
			expectedFailureEvent,
			receivedFailureEvent,
		)
	}
}

func TestGenerateExecutorOnStartEvent(t *testing.T) {

	expectedStartEvent := map[string]any{
		"event":      "executor_on_start",
		"uuid":       seededUuidString,
		"counter":    -1,
		"stdout":     "",
		"start_line": 0,
		"end_line":   0,
		"event_data": map[string]any{
			"crc_dispatcher_correlation_id": "dcdc7b28-6800-4af9-983a-60fda58a7156",
		},
	}
	receivedStartEvent := GenerateExecutorOnStartEvent(
		"dcdc7b28-6800-4af9-983a-60fda58a7156",
		mockUuid,
	)

	if !reflect.DeepEqual(expectedStartEvent, receivedStartEvent) {
		t.Errorf(
			"EXPECTED: %v\nRECEIVED: %v",
			expectedStartEvent,
			receivedStartEvent,
		)
	}
}
