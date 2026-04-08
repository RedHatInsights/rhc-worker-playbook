package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/redhatinsights/rhc-worker-playbook/internal/ansible"
	"github.com/redhatinsights/rhc-worker-playbook/internal/config"
	"github.com/redhatinsights/yggdrasil/worker"
	"github.com/subpop/go-log"
)

func rx(
	w *worker.Worker,
	addr string,
	id string,
	responseTo string,
	metadata map[string]string,
	data []byte,
) error {
	log.Infof("message received: message-id=%v", id)
	defer log.Infof("message finished: message-id=%v", id)

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

	// Get responseInterval from metadata, conditionally overriding it with the
	// value loaded from the configuration file.
	responseIntervalString, has := metadata["response_interval"]
	if !has {
		log.Warn("metadata missing response_interval, defaulting to 300")
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
	// Create the playbook runner and run the playbook
	err = ansible.NewRunner(correlationId, events).Run(data)

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

// verifyPlaybook calls out via subprocess to insights-client's
// ansible.playbook_verifier Python module, passes data as the process's
// standard input. If the playbook passes verification, the playbook, stripped
// of "insights_signature" variables is returned.
func verifyPlaybook(data []byte) ([]byte, error) {

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
	log.Info("Playbook verified.")

	// Register a custom unmarshaler to support the YAML 1.1 boolean types
	// "yes/no" and "on/off".
	yaml.RegisterCustomUnmarshaler[bool](func(b1 *bool, b2 []byte) error {
		if strings.ToLower(string(b2)) == "yes" || strings.ToLower(string(b2)) == "on" ||
			strings.ToLower(string(b2)) == "true" {
			*b1 = true
		} else {
			*b1 = false
		}
		return nil
	})

	// Register a custom marshaler to support the YAML 1.1 boolean types
	// "yes/no" and "on/off".
	yaml.RegisterCustomMarshaler[bool](func(b bool) ([]byte, error) {
		if b {
			return []byte("yes"), nil
		}
		return []byte("no"), nil
	})

	type Playbook struct {
		Name   string                 `yaml:"name"`
		Hosts  string                 `yaml:"hosts"`
		Become bool                   `yaml:"become"`
		Vars   map[string]interface{} `yaml:"vars"`
		Tasks  []yaml.MapSlice        `yaml:"tasks"`
	}
	var playbooks []Playbook
	if err := yaml.UnmarshalWithOptions(stdout, &playbooks); err != nil {
		return nil, fmt.Errorf("cannot unmarshal playbook: %v", err)
	}
	// ansible-runner returns errors when handed binary field values, so
	// remove it before handing off the playbook to ansible-runner.
	delete(playbooks[0].Vars, "insights_signature")

	playbookData, err := yaml.MarshalWithOptions(playbooks, yaml.IndentSequence(false))
	if err != nil {
		return nil, fmt.Errorf("cannot marshal playbook: %v", err)
	}

	return playbookData, nil
}
