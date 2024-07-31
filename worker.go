package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"git.sr.ht/~spc/go-log"
	"github.com/goccy/go-yaml"
	"github.com/redhatinsights/rhc-worker-playbook/internal/ansible"
	"github.com/redhatinsights/rhc-worker-playbook/internal/config"
	"github.com/redhatinsights/rhc-worker-playbook/internal/exec"
	"github.com/redhatinsights/yggdrasil/ipc"
	"github.com/redhatinsights/yggdrasil/worker"
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

	returnURL, has := metadata["return_url"]
	if !has {
		return fmt.Errorf("invalid metadata: missing return_url")
	}

	var correlationID string
	if config.DefaultConfig.VerifyPlaybook {
		var has bool
		correlationID, has = metadata["crc_dispatcher_correlation_id"]
		if !has {
			return fmt.Errorf("invalid metadata: missing crc_dispatcher_correlation_id")
		}
	}

	if config.DefaultConfig.VerifyPlaybook {
		d, err := verifyPlaybook(data, config.DefaultConfig.InsightsCoreGPGCheck)
		if err != nil {
			return fmt.Errorf("cannot verify playbook: %v", err)
		}
		data = d
	}

	events, err := ansible.RunPlaybook(id, data, correlationID)
	if err != nil {
		return fmt.Errorf("cannot run playbook: %v", err)
	}

	// start a goroutine that receives ansible-runner events as they are
	// emitted.
	go func() {
		// TODO: support metadata["response_interval"] batch processing
		for event := range events {
			log.Debugf("ansible-runner event: %v", event)

			responseData, err := json.Marshal(event)
			if err != nil {
				log.Errorf("cannot marshal event: %v", err)
				continue
			}

			err = w.EmitEvent(ipc.WorkerEventNameWorking, event.UUID, id, map[string]string{
				"message": string(responseData),
			})
			if err != nil {
				log.Errorf("cannot emit event: event=%v error=%v", ipc.WorkerEventNameWorking, err)
				continue
			}

			_, _, _, err = w.Transmit(
				returnURL,
				event.UUID,
				id,
				map[string]string{},
				responseData,
			)
			if err != nil {
				log.Errorf("cannot transmit data: %v", err)
				continue
			}
		}
	}()

	return nil
}

// verifyPlaybook calls out via subprocess to insights-client's
// ansible.playbook_verifier Python module, passes data as the process's
// standard input. If the playbook passes verification, the playbook, stripped
// of "insights_signature" variables is returned.
func verifyPlaybook(data []byte, insightsCoreGPGCheck bool) ([]byte, error) {
	env := []string{
		"PATH=/sbin:/bin:/usr/sbin:/usr/bin",
	}
	args := []string{
		"-m",
		"insights.client.apps.ansible.playbook_verifier",
		"--quiet",
		"--payload",
		"noop",
		"--content-type",
		"noop",
	}
	if !insightsCoreGPGCheck {
		args = append(args, "--no-gpg")
		env = append(env, "BYPASS_GPG=True")
	}
	stdin := bytes.NewReader(data)
	stdout, stderr, code, err := exec.RunProcess("/usr/bin/insights-client", args, env, stdin)
	if err != nil {
		log.Debugf(
			"cannot verify playbook: code=%v stdout=%v stderr=%v",
			code,
			string(stdout),
			string(stderr),
		)
		return nil, fmt.Errorf("cannot verify playbook: %v", err)
	}

	if code > 0 {
		return nil, fmt.Errorf("playbook verification failed: %v", string(stderr))
	}

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
