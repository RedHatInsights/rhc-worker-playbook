package config

const (
	FlagNameDirective            = "directive"
	FlagNameInsightsCoreGPGCheck = "insights-core-gpg-check"
	FlagNameLogLevel             = "log-level"
	FlagNameVerifyPlaybook       = "verify-playbook"
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
}

// DefaultConfig is a globally accessible Config data structure, initialized
// with default values.
var DefaultConfig = Config{
	Directive:            "rhc_worker_playbook",
	InsightsCoreGPGCheck: true,
	LogLevel:             "error",
	VerifyPlaybook:       true,
}
