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

// createUuidFunc is a function that returns a UUID, typically uuid.New(),
// used as a function parameter to decouple uuid generation from function logic
type createUuidFunc func() uuid.UUID

type EventManager struct {
	messageId              string
	correlationId          string
	returnURL              string
	responseInterval       time.Duration
	worker                 *worker.Worker
	cachedEvents           []json.RawMessage
	cachedEventsLock       sync.RWMutex
	stopTransmittingEvents chan struct{}
	events                 chan json.RawMessage
}

func NewEventManager(
	messageId string,
	correlationId,
	returnURL string,
	responseInterval time.Duration,
	worker *worker.Worker,
	events chan json.RawMessage,
	stopTransmittingEvents chan struct{},
) *EventManager {
	return &EventManager{
		messageId:              messageId,
		correlationId:          correlationId,
		returnURL:              returnURL,
		responseInterval:       responseInterval,
		worker:                 worker,
		cachedEvents:           []json.RawMessage{},
		stopTransmittingEvents: stopTransmittingEvents,
		events:                 events,
	}
}

// processEvents receives values from the runner and caches them for future use.
func (e *EventManager) ProcessEvents(done chan struct{}) {
	defer close(done)
	for event := range e.events {
		e.cachedEventsLock.Lock()
		e.cachedEvents = append(e.cachedEvents, event)
		e.cachedEventsLock.Unlock()
	}
}

// transmitCachedEvents periodically transmits a batch of cached events when the
// response interval timeout elapses.
func (e *EventManager) TransmitCachedEvents(done chan struct{}) {
	defer close(done)
	timeout := time.Tick(e.responseInterval)
	batchStart := 0
	for {
		select {
		case <-e.stopTransmittingEvents:
			// Transmit one final batch of all events.
			if err := e.transmitEvents(e.cachedEvents); err != nil {
				log.Errorf("cannot transmit events: err=%v", err)
			}
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
			if err := e.transmitEvents(cachedEvents); err != nil {
				log.Errorf("cannot transmit events: err=%v", err)
				continue
			}

			batchStart = batchEnd
		}
	}
}

// transmitEvents sends a slice of json.RawMessage values as an HTTP multipart
// request body and sends it via a D-Bus
// com.redhat.Yggdrasil1.Dispatcher1.Transmit method call.
func (e *EventManager) transmitEvents(events []json.RawMessage) error {
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
		e.messageId,
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

// sendExecutorOnStartEvent generates an executor_on_start event and sends it on the Events channel
func (e *EventManager) SendExecutorOnStartEvent() error {
	event := generateExecutorOnStartEvent(e.correlationId, uuid.New)
	return e.sendExecutorEvent(event)
}

// sendExecutorOnFailedEvent generates an executor_on_failed event and sends it on the Events channel
func (e *EventManager) SendExecutorOnFailedEvent(errorKey string, errorDetails error) error {
	event := generateExecutorOnFailedEvent(
		e.correlationId,
		errorKey,
		errorDetails,
		uuid.New)
	return e.sendExecutorEvent(event)
}

// sendExecutorEvent marshals an event and sends it on the Events channel
func (e *EventManager) sendExecutorEvent(event map[string]any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("cannot marshal JSON: err=%w", err)
	}
	e.events <- json.RawMessage(data)
	return nil
}

// generateExecutorOnStartEvent creates a special executor_on_start event
// to inform Insights that the Ansible job is beginning.
func generateExecutorOnStartEvent(
	correlationID string,
	uuidNew createUuidFunc,
) map[string]any {
	return map[string]any{
		"event":      "executor_on_start",
		"uuid":       uuidNew().String(),
		"counter":    -1,
		"stdout":     "",
		"start_line": 0,
		"end_line":   0,
		"event_data": map[string]any{
			"crc_dispatcher_correlation_id": correlationID,
		},
	}
}

// generateExecutorOnFailedEvent creates a special executor_on_failed event
// to inform Insights that the Ansible job failed to run.
func generateExecutorOnFailedEvent(
	correlationID string,
	errorCode string,
	errorDetails error,
	uuidNew createUuidFunc,
) map[string]any {
	return map[string]any{
		"event":      "executor_on_failed",
		"uuid":       uuidNew().String(),
		"counter":    -1,
		"start_line": 0,
		"end_line":   0,
		"event_data": map[string]any{
			"crc_dispatcher_correlation_id": correlationID,
			"crc_dispatcher_error_code":     errorCode,
			"crc_dispatcher_error_details":  errorDetails.Error(),
		},
	}
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
