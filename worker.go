package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"

	"git.sr.ht/~spc/go-log"
	"github.com/goccy/go-yaml"
	"github.com/google/uuid"
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
		var cachedEvents []byte
		for event := range events {
			log.Debugf("ansible-runner event: %s", event)

			cachedEvents = append(cachedEvents, append(event, '\n')...)

			var runnerEvent map[string]interface{}
			if err := json.Unmarshal(event, &runnerEvent); err != nil {
				log.Errorf("cannot unmarshal JSON: %v", err)
				continue
			}

			eventUUID, ok := runnerEvent["uuid"]
			if !ok {
				log.Errorf("runner event missing UUID: %+v", runnerEvent)
				continue
			}
			err = w.EmitEvent(ipc.WorkerEventNameWorking, eventUUID.(string), id, map[string]string{
				"message": string(event),
			})
			if err != nil {
				log.Errorf("cannot emit event: event=%v error=%v", ipc.WorkerEventNameWorking, err)
				continue
			}

		}

		requestBody, outerContentType, err := buildRequestBody(
			string(cachedEvents),
			"runner-events",
		)
		if err != nil {
			log.Errorf("cannot build request body: event=%+v error=%v", cachedEvents, err)
			return
		}

		responseCode, responseMetadata, responseBody, err := w.Transmit(
			returnURL,
			uuid.New().String(),
			id,
			map[string]string{
				"Content-Type": outerContentType,
			},
			requestBody.Bytes(),
		)
		if err != nil {
			log.Errorf("cannot transmit data: %v", err)
			return
		}
		log.Debugf(
			"received response: code=%v responseMetadata=%v",
			responseCode,
			responseMetadata,
		)
		log.Tracef("responseBody=%v", string(responseBody))

		log.Infof("finished message: message-id=%v", id)
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

// buildRequestBody assembles a multipart/mixed HTTP request body suitable for
// uploading to ingress.
func buildRequestBody(body string, filename string) (*bytes.Buffer, string, error) {
	requestBody := &bytes.Buffer{}
	writer := multipart.NewWriter(requestBody)
	defer writer.Close()

	// Set the inner content-type accordingly, as required by ingress.
	// https://github.com/RedHatInsights/insights-ingress-go/blob/ada891f3dff3f402e4c03ef8aa3a34908cc0a4dc/README.md?plain=1#L46
	innerContentType := "application/vnd.redhat.playbook.v1+jsonl"
	contentDisposition := fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", contentDisposition)
	h.Set("Content-Type", innerContentType)

	part, err := writer.CreatePart(h)
	if err != nil {
		return nil, "", fmt.Errorf("cannot create form part: %v", err)
	}

	_, err = io.WriteString(part, body)
	if err != nil {
		return nil, "", fmt.Errorf("cannot write body to form part: %v", err)
	}

	outerContentType := fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary())

	return requestBody, outerContentType, nil
}
