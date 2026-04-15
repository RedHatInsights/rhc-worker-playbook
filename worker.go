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
	"github.com/google/uuid"
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

	// Get returnURL from message metadata
	returnURL, has := metadata["return_url"]
	if !has {
		return fmt.Errorf("invalid metadata: missing return_url")
	}

	// Get correlationID from metadata
	correlationID, has := metadata["crc_dispatcher_correlation_id"]
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

	// Create the event manager.
	eventManager := ansible.NewEventManager(id, returnURL, responseInterval, w)

	if config.DefaultConfig.VerifyPlaybook {
		d, err := verifyPlaybook(data)
		if err != nil {
			verifyPlaybookError := err

			// If playbook verification fails, send the error back to insights
			// An executor_on_start event is needed since this is prior to Runner initialization
			startEvent := ansible.GenerateExecutorOnStartEvent(correlationID, uuid.New)
			failureEvent := ansible.GenerateExecutorOnFailedEvent(
				correlationID,
				"ANSIBLE_PLAYBOOK_SIGNATURE_VALIDATION_FAILED",
				verifyPlaybookError,
				uuid.New,
			)

			startEventJsonString, err := json.Marshal(startEvent)
			// combine errors and return if JSON serialization fails
			if err != nil {
				return errors.Join(
					verifyPlaybookError,
					fmt.Errorf("cannot marshal JSON: err=%w", err),
				)
			}

			failureEventJsonString, err := json.Marshal(failureEvent)
			// combine errors and return if JSON serialization fails
			if err != nil {
				return errors.Join(
					verifyPlaybookError,
					fmt.Errorf("cannot marshal JSON: err=%w", err),
				)
			}

			if err := eventManager.TransmitEvents(
				[]json.RawMessage{
					json.RawMessage(startEventJsonString),
					json.RawMessage(failureEventJsonString),
				}); err != nil {
				// combine errors and return if transmit fails
				return errors.Join(
					verifyPlaybookError,
					fmt.Errorf("cannot transmit events: err=%w", err),
				)
			}

			return verifyPlaybookError
		}
		data = d
	}

	// Create the playbook runner.
	runner := ansible.NewRunner(correlationID)

	// Start the goroutine processing events from the runner.
	go eventManager.ProcessEvents(runner)

	// Run the playbook.
	err = runner.Run(data)
	if err != nil {
		return fmt.Errorf("cannot run playbook: err=%w", err)
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
