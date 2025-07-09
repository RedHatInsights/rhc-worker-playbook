// Sourced from https://github.com/RedHatInsights/playbook-dispatcher/blob/master/internal/common/model/message/runner.types.gen.go
// Used to narrow the ansible runner event message to only the necessary data

package ansible

type PlaybookRunResponseMessageYamlEventsElem struct {
	// Counter corresponds to the JSON schema field "counter".
	Counter int `json:"counter" yaml:"counter" mapstructure:"counter"`

	// EndLine corresponds to the JSON schema field "end_line".
	EndLine int `json:"end_line" yaml:"end_line" mapstructure:"end_line"`

	// Event corresponds to the JSON schema field "event".
	Event string `json:"event" yaml:"event" mapstructure:"event"`

	// EventData corresponds to the JSON schema field "event_data".
	EventData *PlaybookRunResponseMessageYamlEventsElemEventData `json:"event_data,omitempty" yaml:"event_data,omitempty" mapstructure:"event_data,omitempty"`

	// StartLine corresponds to the JSON schema field "start_line".
	StartLine int `json:"start_line" yaml:"start_line" mapstructure:"start_line"`

	// Stdout corresponds to the JSON schema field "stdout".
	Stdout *string `json:"stdout,omitempty" yaml:"stdout,omitempty" mapstructure:"stdout,omitempty"`

	// Uuid corresponds to the JSON schema field "uuid".
	Uuid string `json:"uuid" yaml:"uuid" mapstructure:"uuid"`
}

type PlaybookRunResponseMessageYamlEventsElemEventData struct {
	// CrcDispatcherCorrelationId corresponds to the JSON schema field
	// "crc_dispatcher_correlation_id".
	CrcDispatcherCorrelationId *string `json:"crc_dispatcher_correlation_id,omitempty" yaml:"crc_dispatcher_correlation_id,omitempty" mapstructure:"crc_dispatcher_correlation_id,omitempty"`

	// CrcDispatcherErrorCode corresponds to the JSON schema field
	// "crc_dispatcher_error_code".
	CrcDispatcherErrorCode *string `json:"crc_dispatcher_error_code,omitempty" yaml:"crc_dispatcher_error_code,omitempty" mapstructure:"crc_dispatcher_error_code,omitempty"`

	// CrcDispatcherErrorDetails corresponds to the JSON schema field
	// "crc_dispatcher_error_details".
	CrcDispatcherErrorDetails *string `json:"crc_dispatcher_error_details,omitempty" yaml:"crc_dispatcher_error_details,omitempty" mapstructure:"crc_dispatcher_error_details,omitempty"`

	// Host corresponds to the JSON schema field "host".
	Host *string `json:"host,omitempty" yaml:"host,omitempty" mapstructure:"host,omitempty"`

	// Playbook corresponds to the JSON schema field "playbook".
	Playbook *string `json:"playbook,omitempty" yaml:"playbook,omitempty" mapstructure:"playbook,omitempty"`

	// PlaybookUuid corresponds to the JSON schema field "playbook_uuid".
	PlaybookUuid *string `json:"playbook_uuid,omitempty" yaml:"playbook_uuid,omitempty" mapstructure:"playbook_uuid,omitempty"`
}
