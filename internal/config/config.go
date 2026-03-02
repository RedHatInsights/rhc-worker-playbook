package config

import (
	"time"

	"github.com/redhatinsights/rhc-worker-playbook/internal/constants"
)

const (
	FlagNameDirective        = "directive"
	FlagNameLogLevel         = "log-level"
	FlagNameVerifyPlaybook   = "verify-playbook"
	FlagNameResponseInterval = "response-interval"
	FlagNameBatchEvents      = "batch-events"
	FlagNameCertFile         = "cert-file"
	FlagNameKeyFile          = "key-file"
	FlagNameCaRoot           = "ca-root"
	FlagNameDataHost         = "data-host"
	FlagNameHTTPRetries      = "http-retries"
	FlagNameHTTPTimeout      = "http-timeout"
)

type Config struct {
	// Directive is the worker destination name to register with yggdrasil.
	Directive string

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

	// CertFile is a path to a public certificate, optionally used along with
	// KeyFile to authenticate connections.
	CertFile string

	// KeyFile is a path to a private certificate, optionally used along with
	// CertFile to authenticate connections.
	KeyFile string

	// CARoot is the list of paths with chain certificate file to optionally
	// include in the TLS configration's CA root list.
	CARoot []string

	// DataHost is a hostname value to interject into all HTTP requests when
	// handling data retrieval.
	DataHost string

	// HTTPRetries is the number of times the client will attempt to resend
	// failed HTTP requests before giving up.
	HTTPRetries int

	// HTTPTimeout is the duration the client will wait before cancelling an
	// HTTP request.
	HTTPTimeout time.Duration
}

// DefaultConfig is a globally accessible Config data structure, initialized
// with default values.
var DefaultConfig = Config{
	Directive:        "rhc_worker_playbook",
	LogLevel:         "error",
	VerifyPlaybook:   true,
	ResponseInterval: 0,
	BatchEvents:      0,
	CertFile:         "/etc/pki/consumer/cert.pem",
	KeyFile:          "/etc/pki/consumer/key.pem",
	CARoot:           []string{},
	DataHost:         constants.DefaultDataHost,
	HTTPRetries:      0,
	HTTPTimeout:      0,
}
