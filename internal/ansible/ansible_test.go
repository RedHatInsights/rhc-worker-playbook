package ansible

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"git.sr.ht/~spc/go-log"
)

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

	filteredJobEventData := filterJobEvent(sampleJobEventData)

	// should be filtered (different) since attributes were reduced
	if reflect.DeepEqual(filteredJobEventData, sampleJobEventData) {
		t.Errorf(
			"EXPECTED: %v\nRECEIVED: %v",
			string(sampleJobEventData),
			string(filteredJobEventData),
		)
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

	// capture stderr
	var buf bytes.Buffer
	log.SetOutput(&buf)
	filteredJobEventData := filterJobEvent(sampleJobEventData)

	// should be identical
	if !reflect.DeepEqual(filteredJobEventData, sampleJobEventData) {
		t.Errorf(
			"EXPECTED: %v\nRECEIVED: %v",
			string(sampleJobEventData),
			string(filteredJobEventData),
		)
	}

	// error should be logged
	if !strings.Contains(buf.String(), "error filtering job event") {
		t.Errorf("Filtering error was not logged: %v.", buf.String())
	}
}
