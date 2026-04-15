package ansible

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

	"github.com/google/uuid"
	"github.com/redhatinsights/rhc-worker-playbook/internal/config"
	"github.com/redhatinsights/yggdrasil/worker"
	"github.com/subpop/go-log"
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

// ProcessEvents receives values from the runner and caches them for future use.
func (e *EventManager) ProcessEvents(runner *Runner) {
	go e.transmitCachedEvents()

	for event := range runner.Events {
		e.cachedEventsLock.Lock()
		e.cachedEvents = append(e.cachedEvents, event)
		e.cachedEventsLock.Unlock()
	}

	// Signal the sending events goroutine to stop.
	e.stopSendingEvents <- struct{}{}

	// Transmit one final batch of all events.
	if err := e.TransmitEvents(e.cachedEvents); err != nil {
		log.Errorf("cannot transmit events: err=%v", err)
	}

	log.Infof("message finished: message-id=%v", e.id)
}

// TransmitCachedEvents periodically transmits a batch of cached events when the
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

			// If the value of the current batch start has caught up to the
			// known end of the cached events and the timeout has triggered
			// again, skip this iteration.
			if batchStart >= batchEnd {
				e.cachedEventsLock.RUnlock()
				continue
			}

			cachedEvents := append([]json.RawMessage{}, e.cachedEvents[batchStart:batchEnd]...)
			e.cachedEventsLock.RUnlock()
			log.Debugf(
				"transmitting cached events: batchStart=%v batchEnd=%v",
				batchStart,
				batchEnd,
			)
			if err := e.TransmitEvents(cachedEvents); err != nil {
				log.Errorf("cannot transmit events: err=%v", err)
				continue
			}

			batchStart = batchEnd
		}
	}
}

// TransmitEvents sends a slice of json.RawMessage values as an HTTP multipart
// request body and sends it via a D-Bus
// com.redhat.Yggdrasil1.Dispatcher1.Transmit method call.
func (e *EventManager) TransmitEvents(events []json.RawMessage) error {
	// Build a JSONL data buffer.
	body := strings.Builder{}
	for _, event := range events {
		_, err := body.Write(event)
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

	if responseCode >= 400 {
		// return an error if HTTP status code is 400 and up
		return fmt.Errorf(
			"server returned error response: code=%v responseMetadata=%v responseBody=%v",
			responseCode,
			responseMetadata,
			string(responseBody),
		)
	}

	return nil
}

// buildRequestBody assembles a multipart/mixed HTTP request body suitable for
// uploading to ingress.
func buildRequestBody(body string, filename string) (*bytes.Buffer, string, error) {
	requestBody := &bytes.Buffer{}
	writer := multipart.NewWriter(requestBody)
	defer func() {
		closeErr := writer.Close()
		if closeErr != nil {
			log.Errorf("cannot close request body writer: %v", closeErr)
		}
	}()

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
