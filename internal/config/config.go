package config

import "time"

const (
	FlagNameDirective            = "directive"
	FlagNameInsightsCoreGPGCheck = "insights-core-gpg-check"
	FlagNameLogLevel             = "log-level"
	FlagNameVerifyPlaybook       = "verify-playbook"
	FlagNameResponseInterval     = "response-interval"
	FlagNameBatchEvents          = "batch-events"
)

type Config struct {
	// Directive is the worker destination name to register with yggdrasil.
	Directive string

	// InsightsCoreGPGCheck determines whether or not to perform GPG
	// verification on insights-core.egg.
	InsightsCoreGPGCheck bool

	// LogLevel is the level value used for logging.
	LogLevel string

	// VerifyPlaybook determines whether or not to verify incoming playbooks'
	// GPG signatures.
	VerifyPlaybook bool

	// ResponseInterval overrides the response interval value set in the
	// message, instead always setting it to this value.
	ResponseInterval time.Duration

	// BatchEvents is the number of events to batch together in a given transmit
	// response.
	BatchEvents int
}

// DefaultConfig is a globally accessible Config data structure, initialized
// with default values.
var DefaultConfig = Config{
	Directive:            "rhc_worker_playbook",
	InsightsCoreGPGCheck: true,
	LogLevel:             "error",
	VerifyPlaybook:       true,
	ResponseInterval:     0,
	BatchEvents:          0,
}
