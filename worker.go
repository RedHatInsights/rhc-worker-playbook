package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/google/uuid"
	"github.com/redhatinsights/rhc-worker-playbook/internal/ansible"
	"github.com/redhatinsights/rhc-worker-playbook/internal/config"
	"github.com/redhatinsights/yggdrasil/worker"
)

type Play struct {
	Name   string          `yaml:"name"`
	Hosts  string          `yaml:"hosts"`
	Become *bool           `yaml:"become,omitempty"`
	Vars   map[string]any  `yaml:"vars"`
	Tasks  []yaml.MapSlice `yaml:"tasks"`
}

var playbookAlreadyRunning sync.Mutex

func init() {
	// Register a custom unmarshaler to support the YAML 1.1 boolean types
	// "yes/no" and "on/off".

	// full spec: https://yaml.org/type/bool.html
	// allowed values:
	//  y|Y|yes|Yes|YES|n|N|no|No|NO
	// |true|True|TRUE|false|False|FALSE
	// |on|On|ON|off|Off|OFF
	yaml.RegisterCustomUnmarshaler[bool](func(b1 *bool, b2 []byte) error {
		toString := strings.TrimSpace(string(b2))
		switch toString {
		case "y", "Y", "yes", "Yes", "YES", "true", "True", "TRUE", "on", "On", "ON":
			*b1 = true
		case "n", "N", "no", "No", "NO", "false", "False", "FALSE", "off", "Off", "OFF":
			*b1 = false
		default:
			return fmt.Errorf("unable to parse boolean: %v", toString)
		}
		return nil
	})
}

func rx(
	w *worker.Worker,
	addr string,
	id string,
	responseTo string,
	metadata map[string]string,
	data []byte,
) error {
	slog.Info("message received:", "message-id", id)
	defer slog.Info("message finished:", "message-id", id)

	// Get returnURL from message metadata
	returnURL, has := metadata["return_url"]
	if !has {
		return fmt.Errorf("invalid metadata: missing return_url")
	}

	// Get correlationID from metadata
	correlationId, has := metadata["crc_dispatcher_correlation_id"]
	if !has {
		return fmt.Errorf("invalid metadata: missing crc_dispatcher_correlation_id")
	}

	// Verify correlationID is a UUID
	if err := uuid.Validate(correlationId); err != nil {
		return fmt.Errorf("invalid UUID: %s", correlationId)
	}

	// Get responseInterval from metadata, conditionally overriding it with the
	// value loaded from the configuration file.
	responseIntervalString, has := metadata["response_interval"]
	if !has {
		slog.Warn("metadata missing response_interval, defaulting to 300")
		responseIntervalString = "300"
	}
	responseInterval, err := time.ParseDuration(responseIntervalString + "s")
	if err != nil {
		return fmt.Errorf("cannot parse response interval: err=%w", err)
	}
	if config.DefaultConfig.ResponseInterval > 0 {
		responseInterval = config.DefaultConfig.ResponseInterval
	}

	// Adjust responseInterval for batching mode.
	if config.DefaultConfig.BatchEvents > 0 {
		// Set the response interval to 500ms when batching events. This has the
		// effect of matching the "<-timeout" case every time the channel select
		// statement evaluates. This allows the same codepath to work when
		// either batching events by quantity or by timeout.
		responseInterval = 500 * time.Millisecond
	}

	// events is a channel for communication between the Runner and EventManager goroutines
	// Runner sends job events, and EventManager receives them
	events := make(chan json.RawMessage)

	// stopTransmittingEvents is a channel to signal to TransmitCachedEvents to finish
	stopTransmittingEvents := make(chan struct{})
	eventManager := ansible.NewEventManager(
		id,
		correlationId,
		returnURL,
		responseInterval,
		w,
		events,
		stopTransmittingEvents,
	)

	// Start the goroutine processing events from the runner
	processEventsDone := make(chan struct{})
	go eventManager.ProcessEvents(processEventsDone)

	// Start the goroutine to transmit the set of cached events back to yggdrasil
	transmitCachedEventsDone := make(chan struct{})
	go eventManager.TransmitCachedEvents(transmitCachedEventsDone)

	// Channel and goroutine teardown
	defer func() {
		// Close the events channel, wait processEvents to do any final writes
		close(events)
		<-processEventsDone

		// End transmitCachedEvents, wait for the last transmit
		close(stopTransmittingEvents)
		<-transmitCachedEventsDone
	}()

	// Publish an "executor_on_start" event to signal cloud connector that a run
	// event has started
	if err := eventManager.SendExecutorOnStartEvent(); err != nil {
		return err
	}

	// Try and lock the mutex.
	// If the lock is successful, continue.
	// If the lock is unsuccessful, a playbook is already running. Send an error to remediations and exit.
	if !playbookAlreadyRunning.TryLock() {
		// playbook is currently in progress
		playbookAlreadyRunningErr := errors.New(
			"a playbook run is already in progress, please wait until the current playbook finishes before executing another",
		)

		if err := eventManager.SendExecutorOnFailedEvent(
			"ANSIBLE_PLAYBOOK_ALREADY_RUNNING",
			playbookAlreadyRunningErr,
		); err != nil {
			return errors.Join(playbookAlreadyRunningErr, err)
		}

		return playbookAlreadyRunningErr
	}

	// Unlock the mutex after the playbook run
	defer playbookAlreadyRunning.Unlock()

	// Verify the playbook
	if config.DefaultConfig.VerifyPlaybook {
		data, err = verifyPlaybook(data)
		if err != nil {
			verifyPlaybookError := err

			if err := eventManager.SendExecutorOnFailedEvent(
				"ANSIBLE_PLAYBOOK_SIGNATURE_VALIDATION_FAILED",
				verifyPlaybookError,
			); err != nil {
				return errors.Join(verifyPlaybookError, err)
			}

			return verifyPlaybookError
		}
	}

	// Strip the signature - also verifies the data is YAML
	data, err = stripSignature(data)
	if err != nil {
		stripSignatureError := err

		if err := eventManager.SendExecutorOnFailedEvent(
			"ANSIBLE_YAML_VALIDATION_FAILED",
			stripSignatureError,
		); err != nil {
			return errors.Join(stripSignatureError, err)
		}

		return stripSignatureError
	}

	// Create the playbook runner and run the playbook
	runner, err := ansible.NewRunner(correlationId, events)

	if err == nil {
		err = runner.Run(data)
	}

	if err != nil {
		playbookRunError := fmt.Errorf("cannot run playbook: err=%w", err)

		if err := eventManager.SendExecutorOnFailedEvent(
			"UNDEFINED_ERROR",
			playbookRunError,
		); err != nil {
			return errors.Join(playbookRunError, err)
		}

		return playbookRunError
	}

	return nil
}

// verifyPlaybook calls out via subprocess to rhc-playbook-verifier,
// and passes data as the process's standard input.
// If the playbook passes verification, the stdout
// of rhc-playbook-verifier is returned
func verifyPlaybook(data []byte) ([]byte, error) {
	slog.Info("verifying playbook")

	stdin := bytes.NewReader(data)
	stdoutb := new(bytes.Buffer)
	stderrb := new(bytes.Buffer)

	rhcPlaybookVerifierCmd := exec.Command(
		"/usr/libexec/rhc-playbook-verifier",
		"--stdin",
	)
	rhcPlaybookVerifierCmd.Env = []string{
		"PATH=/sbin:/bin:/usr/sbin:/usr/bin",
	}
	rhcPlaybookVerifierCmd.Stdin = stdin
	rhcPlaybookVerifierCmd.Stdout = stdoutb
	rhcPlaybookVerifierCmd.Stderr = stderrb

	slog.Info("launching rhc-playbook-verifier subprocess")
	slog.Debug("launching with parameters:",
		"args", rhcPlaybookVerifierCmd.Args,
		"env", rhcPlaybookVerifierCmd.Env,
		"stdin", string(data))

	err := rhcPlaybookVerifierCmd.Run()

	code := rhcPlaybookVerifierCmd.ProcessState.ExitCode()
	stdout := stdoutb.Bytes()
	stderr := stderrb.Bytes()

	if err != nil {
		verifyPlaybookError := fmt.Errorf(
			"cannot verify playbook: code=%v stdout=%v stderr=%v",
			code,
			string(stdout),
			string(stderr),
		)
		return nil, verifyPlaybookError
	}

	// verification succeeds, log here
	slog.Info("playbook verified.")

	return stdout, nil
}

// stripSignature will confirm the playbook is YAML and return
// the playbook stripped of "insights_signature" variables
func stripSignature(data []byte) ([]byte, error) {
	var plays []Play
	if err := yaml.UnmarshalWithOptions(data, &plays); err != nil {
		return nil, fmt.Errorf("cannot unmarshal playbook: %v", err)
	}

	// ansible-runner returns errors when handed binary field values, so
	// remove the signatures from the plays before handing off the playbook
	// to ansible-runner.
	for _, play := range plays {
		delete(play.Vars, "insights_signature")
		delete(play.Vars, "insights_signature_exclude")
	}

	playbookData, err := yaml.MarshalWithOptions(plays, yaml.IndentSequence(false))
	if err != nil {
		return nil, fmt.Errorf("cannot marshal playbook: %v", err)
	}

	return playbookData, nil
}
