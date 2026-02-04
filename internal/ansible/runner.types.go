// Sourced from https://github.com/RedHatInsights/playbook-dispatcher/blob/master/internal/common/model/message/runner.types.gen.go
// Used to narrow the ansible runner event message to only the necessary data

package ansible

// Overall structure of the data in an ansible job event needed by playbook dispatcher
type PlaybookRunResponseMessageEventsElem struct {
	// Counter corresponds to the JSON schema field "counter".
	Counter int `json:"counter"`

	// EndLine corresponds to the JSON schema field "end_line".
	EndLine int `json:"end_line"`

	// Event corresponds to the JSON schema field "event".
	Event string `json:"event"`

	// EventData corresponds to the JSON schema field "event_data".
	EventData *PlaybookRunResponseMessageEventsElemEventData `json:"event_data,omitempty"`

	// StartLine corresponds to the JSON schema field "start_line".
	StartLine int `json:"start_line" yaml:"start_line"`

	// Stdout corresponds to the JSON schema field "stdout".
	Stdout *string `json:"stdout,omitempty"`

	// Uuid corresponds to the JSON schema field "uuid".
	Uuid string `json:"uuid"`

	// RunnerIdent corresponds to the JSON schema field "runner_ident"
	// The runner_ident property is not *currently* in the playbook dispatcher
	// schema, but is included here to prepare for future improvements
	// to discoverability of ansible-runner logs on the system.
	// See: https://issues.redhat.com/browse/RHINENG-19062
	RunnerIdent string `json:"runner_ident"`
}

// Structure of the "event_data" field in an ansible job event needed by playbook dispatcher
type PlaybookRunResponseMessageEventsElemEventData struct {
	// CrcDispatcherCorrelationId corresponds to the JSON schema field
	// "crc_dispatcher_correlation_id".
	CrcDispatcherCorrelationId *string `json:"crc_dispatcher_correlation_id,omitempty"`

	// CrcDispatcherErrorCode corresponds to the JSON schema field
	// "crc_dispatcher_error_code".
	CrcDispatcherErrorCode *string `json:"crc_dispatcher_error_code,omitempty"`

	// CrcDispatcherErrorDetails corresponds to the JSON schema field
	// "crc_dispatcher_error_details".
	CrcDispatcherErrorDetails *string `json:"crc_dispatcher_error_details,omitempty"`

	// Host corresponds to the JSON schema field "host".
	Host *string `json:"host,omitempty"`

	// Playbook corresponds to the JSON schema field "playbook".
	Playbook *string `json:"playbook,omitempty"`

	// PlaybookUuid corresponds to the JSON schema field "playbook_uuid".
	PlaybookUuid *string `json:"playbook_uuid,omitempty"`
}
