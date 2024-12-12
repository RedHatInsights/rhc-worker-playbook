package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/goccy/go-yaml"
	"github.com/google/uuid"
	"github.com/redhatinsights/rhc-worker-playbook/internal/ansible"
	"github.com/redhatinsights/rhc-worker-playbook/internal/config"
	"github.com/redhatinsights/rhc-worker-playbook/internal/exec"
	"github.com/redhatinsights/yggdrasil/worker"
)

type EventManager struct {
	id                string
	returnURL         string
	responseInterval  time.Duration
	worker            *worker.Worker
	cachedEvents      []json.RawMessage
	cachedEventsLock  sync.RWMutex
	stopSendingEvents chan struct{}
}

func NewEventManager(
	id string,
	returnURL string,
	responseInterval time.Duration,
	worker *worker.Worker,
) *EventManager {
	return &EventManager{
		id:                id,
		returnURL:         returnURL,
		responseInterval:  responseInterval,
		worker:            worker,
		cachedEvents:      []json.RawMessage{},
		stopSendingEvents: make(chan struct{}),
	}
}

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

	if config.DefaultConfig.VerifyPlaybook {
		d, err := verifyPlaybook(data, config.DefaultConfig.InsightsCoreGPGCheck)
		if err != nil {
			return fmt.Errorf("cannot verify playbook: err=%w", err)
		}
		data = d
	}

	// Create the event manager.
	eventManager := NewEventManager(id, returnURL, responseInterval, w)

	// Create the playbook runner.
	runner := ansible.NewRunner(correlationID, 60*time.Second)

	// Start the goroutine processing events from the runner.
	go eventManager.processEvents(runner)
	go eventManager.transmitCachedEvents()

	// Run the playbook.
	err = runner.Run(data)
	if err != nil {
		return fmt.Errorf("cannot run playbook: err=%w", err)
	}

	return nil
}

// processEvents receives values from the runner and caches them for future use.
func (e *EventManager) processEvents(runner *ansible.Runner) {
	for event := range runner.Events {
		e.cachedEventsLock.Lock()
		e.cachedEvents = append(e.cachedEvents, event)
		e.cachedEventsLock.Unlock()
	}

	// Signal the sending events goroutine to stop.
	e.stopSendingEvents <- struct{}{}

	// Transmit one final batch of all events.
	e.cachedEventsLock.RLock()
	length := len(e.cachedEvents)
	e.cachedEventsLock.RUnlock()
	if err := e.transmitEvents(0, length); err != nil {
		log.Errorf("cannot transmit events: err=%v", err)
	}

	log.Infof("message finished: message-id=%v", e.id)
}

// transmitCachedEvents periodically transmits a batch of cached events when the
// response interval timeout elapses.
func (e *EventManager) transmitCachedEvents() {
	timeout := time.Tick(e.responseInterval)
	batchStart := 0
	for {
		select {
		case <-e.stopSendingEvents:
			return
		case <-timeout:
			var batchEnd int

			e.cachedEventsLock.RLock()
			if config.DefaultConfig.BatchEvents > 0 {
				// If batching events, compute the end of the batch, ensuring
				// the end does not exceed the length of the cached events.
				batchEnd = batchStart + config.DefaultConfig.BatchEvents
				if batchEnd > len(e.cachedEvents) {
					batchEnd = len(e.cachedEvents)
				}
			} else {
				// If not batching events, treat the entire slice as one
				// "batch".
				batchStart = 0
				batchEnd = len(e.cachedEvents)
			}
			e.cachedEventsLock.RUnlock()

			// If the value of the current batch start has caught up to the
			// known end of the cached events and the timeout has triggered
			// again, skip this iteration.
			if batchStart >= batchEnd {
				continue
			}

			log.Debugf(
				"transmitting cached events: batchStart=%v batchEnd=%v",
				batchStart,
				batchEnd,
			)
			if err := e.transmitEvents(batchStart, batchEnd); err != nil {
				log.Errorf("cannot transmit events: err=%v", err)
				continue
			}

			batchStart = batchEnd
		}
	}
}

// transmitEvents sends a subslice of cachedEvents as an HTTP multipart
// request body and sends it via a D-Bus
// com.redhat.Yggdrasil1.Dispatcher1.Transmit method call.
func (e *EventManager) transmitEvents(start, end int) error {
	e.cachedEventsLock.RLock()
	defer e.cachedEventsLock.RUnlock()

	// Build a JSONL data buffer.
	body := strings.Builder{}
	for _, cachedEvent := range e.cachedEvents[start:end] {
		_, err := body.Write(cachedEvent)
		if err != nil {
			return fmt.Errorf("cannot write to body: err=%w", err)
		}
		_ = body.WriteByte('\n')
	}
	requestBody, outerContentType, err := buildRequestBody(
		body.String(),
		"runner-events",
	)
	if err != nil {
		return fmt.Errorf("cannot build request body: err=%v", err)
	}

	responseCode, responseMetadata, responseBody, err := e.worker.Transmit(
		e.returnURL,
		uuid.New().String(),
		e.id,
		map[string]string{
			"Content-Type": outerContentType,
		},
		requestBody.Bytes(),
	)
	if err != nil {
		return fmt.Errorf("cannot transmit data: err=%v", err)
	}
	log.Debugf(
		"received response: code=%v responseMetadata=%v",
		responseCode,
		responseMetadata,
	)
	log.Tracef("responseBody=%v", string(responseBody))

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
