package ansible

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"git.sr.ht/~spc/go-log"
	"github.com/google/uuid"
	"github.com/redhatinsights/rhc-worker-playbook/internal/constants"
	"github.com/redhatinsights/rhc-worker-playbook/internal/exec"
	"github.com/rjeczalik/notify"
)

// RunPlaybook creates an ansible-runner job to run the provided playbook. The
// function returns a channel over which the caller can receive ansible-runner
// events as they happen. The channel is closed when the job completes.
func RunPlaybook(id string, playbook []byte, correlationID string) (chan json.RawMessage, error) {
	privateDataDir := filepath.Join(constants.StateDir, id)

	if err := os.WriteFile(filepath.Join(constants.StateDir, id+".yaml"), playbook, 0600); err != nil {
		return nil, fmt.Errorf("cannot write playbook to temp file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(privateDataDir, "artifacts", id, "job_events"), 0755); err != nil {
		return nil, fmt.Errorf("cannot create private state dir: %v", err)
	}

	// precreate the status file so that we can watch for when it gets written
	// to.
	status, err := os.Create(filepath.Join(privateDataDir, "artifacts", id, "status"))
	if err != nil {
		return nil, fmt.Errorf("cannot create status file: %v", err)
	}
	_ = status.Close()

	// Guard the events channel with a WaitGroup to ensure that all goroutines
	// that need to send to it are finished before closing the channel.
	var wg sync.WaitGroup
	wg.Add(1)
	events := make(chan json.RawMessage)

	// publish an "executor_on_start" event to signal cloud connector that a run
	// event has started. This is run on a goroutine in case the events
	// channel doesn't have a receiver connected yet to avoid blocking the
	// continuation of this function.
	go func() {
		event := map[string]interface{}{
			"event":      "executor_on_start",
			"uuid":       uuid.New().String(),
			"counter":    -1,
			"stdout":     "",
			"start_line": 0,
			"end_line":   0,
			"event_data": map[string]interface{}{
				"crc_dispatcher_correlation_id": correlationID,
			},
		}
		data, err := json.Marshal(event)
		if err != nil {
			log.Errorf("cannot marshal json: %v", err)
			return
		}
		events <- data
	}()

	// started is a closure that's passed to the StartProcess function as its
	// exec.ProcessStartedFunc handler. This function is invoked on a goroutine
	// after the process has been started. It exists as a named closure within
	// this function to capture some relevant values that aren't otherwise
	// passed in as part of the exec.ProcessStartedFunc signature (id,
	// privateDataDir, etc.).
	started := func(pid int, stdout, stderr io.ReadCloser) {
		log.Debugf("process started: pid=%v runner_ident=%v", pid, id)

		// start a goroutine to watch for event files being written to the
		// job_events directory. When a relevant event file is detected, it gets
		// marshaled into JSON and sent to the events channel.
		jobEventsChan := make(chan notify.EventInfo, 1)
		jobEventsDir := filepath.Join(privateDataDir, "artifacts", id, "job_events")
		if err := notify.Watch(jobEventsDir, jobEventsChan, notify.InMovedTo); err != nil {
			log.Errorf("failed to watch job_events dir: dir=%v err=%v", privateDataDir, err)
			return
		}
		defer notify.Stop(jobEventsChan)
		defer close(jobEventsChan)
		go func(c chan notify.EventInfo) {
			log.Tracef("start goroutine watching job_events dir: %v", jobEventsDir)
			defer log.Tracef("stop goroutine watching job_events dir: %v", jobEventsDir)

			for e := range c {
				log.Debugf("notify event: event=%v path=%v", e.Event(), e.Path())
				switch e.Event() {
				case notify.InMovedTo:
					if strings.HasSuffix(e.Path(), ".json") {
						data, err := os.ReadFile(e.Path())
						if err != nil {
							log.Errorf("cannot read file: file=%v error=%v", e.Path(), err)
							continue
						}

						// Unmarshal the event data into an untyped map instead
						// of a strictly typed structure. Using a strictly typed
						// struct has the unintentional side effect of
						// discarding any fields from the ansible-running event
						// JSON that are not explicitly named in a struct. This
						// allows the fields that are immaterial to the worker
						// to still be included in the data structure.
						var event map[string]interface{}
						if err := json.Unmarshal(data, &event); err != nil {
							log.Errorf("cannot unmarshal data: data=%v error=%v", data, err)
							continue
						}
						eventData, ok := event["event_data"]
						if !ok {
							eventData = map[string]interface{}{}
						}
						if _, has := eventData.(map[string]interface{})["crc_dispatcher_correlation_id"]; !has {
							eventData.(map[string]interface{})["crc_dispatcher_correlation_id"] = correlationID
						}
						eventData.(map[string]interface{})["crc_message_version"] = 1
						event["event_data"] = eventData

						// "counter" is a required field according to
						// playbook-dispatcher's openapi schema. Messages
						// without a "counter" field are rejected as invalid by
						// the server.
						// The same is true for "start_line" and "end_line".
						// https://github.com/RedHatInsights/playbook-dispatcher/blob/22853a47c5bb85c94fdb2a645fef02758247d4ae/schema/playbookRunResponse.message.yaml#L58-L63
						if _, has := event["counter"]; !has {
							event["counter"] = -1
						}
						if _, has := event["start_line"]; !has {
							event["start_line"] = 0
						}
						if _, has := event["end_line"]; !has {
							event["end_line"] = 0
						}

						modifiedData, err := json.Marshal(event)
						if err != nil {
							log.Errorf("cannot marshal JSON: %v", err)
							continue
						}

						events <- modifiedData
						log.Infof("event sent: event=%+v", event)
					}
				}
			}
		}(jobEventsChan)

		// start a goroutine that watches for the "status" file. When it gets
		// written to, its contents are read, and if the status is "failed", a
		// final "failed" event is written to the events channel. No action is
		// taken if the status is "successful".
		statusFileChan := make(chan notify.EventInfo, 1)
		statusFilePath := filepath.Join(privateDataDir, "artifacts", id, "status")
		if err := notify.Watch(statusFilePath, statusFileChan, notify.InCloseWrite); err != nil {
			log.Errorf("failed to watch status file %v: %v", statusFilePath, err)
		}
		defer notify.Stop(statusFileChan)
		defer close(statusFileChan)
		go func(c chan notify.EventInfo) {
			defer wg.Done()
			log.Tracef("start goroutine watching status file: %v", statusFilePath)
			defer log.Tracef("stop goroutine watching status file: %v", statusFilePath)

			for e := range c {
				log.Debugf("notify event: event=%v path=%v", e.Event(), e.Path())
				switch e.Event() {
				case notify.InCloseWrite:
					data, err := os.ReadFile(e.Path())
					if err != nil {
						log.Errorf("failed to read status file: %v", err)
						continue
					}
					switch string(data) {
					case "failed":
						// publish an "executor_on_failed" event to signal
						// cloud connector that a run has failed.
						event := map[string]interface{}{
							"event":      "executor_on_failed",
							"uuid":       uuid.New().String(),
							"counter":    -1,
							"start_line": 0,
							"end_line":   0,
							"event_data": map[string]interface{}{
								"crc_dispatcher_correlation_id": correlationID,
								"crc_dispatcher_error_code":     "UNDEFINED_ERROR",
							},
						}

						data, err := json.Marshal(event)
						if err != nil {
							log.Errorf("cannot marshal JSON: %v", err)
							continue
						}
						events <- json.RawMessage(data)
					case "successful":
					default:
						log.Errorf("unsupported status case: %v", string(data))
					}
				}
			}
		}(statusFileChan)

		// Block the remainder of the routine until the process exits. When it
		// does, clean up.
		err = exec.WaitProcess(pid, func(pid int, state *os.ProcessState) {
			log.Debugf("process stopped: pid=%v runner_ident=%v exit=%v", pid, id, state.ExitCode())
			wg.Wait()
			close(events)
		})
		if err != nil {
			log.Errorf("process stopped with error: %v", err)
			return
		}
	}

	err = exec.StartProcess(
		"/usr/bin/python3",
		[]string{
			"-m",
			"ansible_runner",
			"run",
			"--ident",
			id,
			"--json",
			"--playbook",
			filepath.Join(constants.StateDir, id+".yaml"),
			privateDataDir,
		},
		[]string{
			"PATH=/sbin:/bin:/usr/sbin:/usr/bin",
			"PYTHONPATH=" + filepath.Join(constants.LibDir, "rhc-worker-playbook"),
			"PYTHONDONTWRITEBYTECODE=1",
			"ANSIBLE_COLLECTIONS_PATH=" + filepath.Join(
				constants.DataDir,
				"rhc-worker-playbook",
				"ansible",
				"collections",
				"ansible_collections",
			),
		},
		started,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot start process: %v", err)
	}

	return events, nil
}
