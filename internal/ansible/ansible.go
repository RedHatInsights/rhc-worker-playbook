package ansible

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/redhatinsights/rhc-worker-playbook/internal/constants"
	"github.com/rjeczalik/notify"
	"github.com/subpop/go-log"
)

// Runner maintains the state of a playbook run during execution.
type Runner struct {
	// ID is the identity of the job. It is used in file paths and event
	// metadata.
	ID string

	// Events contain runner events, marshaled as raw JSON. Receive values from
	// this channel to receive the current state of the run.
	Events chan json.RawMessage

	// Status is the final status of the run. It will be empty if the job has
	// not completed.
	Status string

	playbookPath   string
	jobEventsPath  string
	statusFilePath string

	stopJobEventsWatch chan struct{}
}

// createUuidFunc is a function that returns a UUID, typically uuid.New(),
// used as a function parameter to decouple uuid generation from function logic
type createUuidFunc func() uuid.UUID

// NewRunner creates a new Runner, uniquely identified by ID.
func NewRunner(ID string) *Runner {
	return &Runner{
		Events:             make(chan json.RawMessage),
		ID:                 ID,
		playbookPath:       filepath.Join(constants.StateDir, ID+".yaml"),
		jobEventsPath:      filepath.Join(constants.PrivateDataDir, "artifacts", ID, "job_events"),
		statusFilePath:     filepath.Join(constants.PrivateDataDir, "artifacts", ID, "status"),
		stopJobEventsWatch: make(chan struct{}),
	}
}

// Run begins running the provided playbook, using the given ID as the run
// identity. It returns after ansible-runner completes the playbook run.
// Events will be sent to the runner's Events channel. When the channel closes,
// the run is complete.
func (r *Runner) Run(playbook []byte) error {
	defer r.end()

	// write playbook to the filesystem
	if err := os.WriteFile(r.playbookPath, playbook, 0600); err != nil {
		return fmt.Errorf("cannot write playbook file: path=%v err=%w", r.playbookPath, err)
	}

	// precreate the job_events directory so that we can watch for when events
	// get written to it.
	if err := os.MkdirAll(r.jobEventsPath, 0755); err != nil {
		return fmt.Errorf(
			"cannot create job_events directory: directory=%v err=%w",
			r.jobEventsPath,
			err,
		)
	}

	// start a goroutine to watch for event files being written to the
	// job_events directory. When a relevant event file is detected, it gets
	// marshaled into JSON and sent to the events channel.
	go r.watchJobEvents()

	// publish an "executor_on_start" event to signal cloud connector that a run
	// event has started. This is run on a goroutine in case the events
	// channel doesn't have a receiver connected yet to avoid blocking the
	// continuation of this function.
	// --
	// @TODO (RHINENG-25480): move this out of the Run function and into worker.go --
	// 	this should happen before anything else so we can log any errors back to remediations
	event := GenerateExecutorOnStartEvent(r.ID, uuid.New)
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("cannot marshal json: err=%v", err)
	}
	r.Events <- data
	// --

	ansibleRunnerCmd := exec.Command(
		"/usr/bin/python3",
		"-m",
		"ansible_runner",
		"run",
		"--ident",
		r.ID,
		"--playbook",
		r.playbookPath,
		constants.PrivateDataDir,
	)
	ansibleRunnerCmd.Env = []string{
		"PATH=/sbin:/bin:/usr/sbin:/usr/bin",
		"PYTHONPATH=" + filepath.Join(constants.LibDir, "rhc-worker-playbook"),
		"PYTHONDONTWRITEBYTECODE=1",
		"ANSIBLE_HOME=" + constants.AnsibleHomePath,
		"ANSIBLE_REMOTE_TMP=" + constants.AnsibleRemoteTmpPath,
		"ANSIBLE_COLLECTIONS_PATH=" + filepath.Join(
			constants.DataDir,
			"rhc-worker-playbook",
			"ansible",
			"collections",
			"ansible_collections",
		),
	}

	if err := ansibleRunnerCmd.Start(); err != nil {
		return fmt.Errorf("cannot start ansible-runner: err=%w", err)
	}

	log.Infof("run started: pid=%v", ansibleRunnerCmd.Process.Pid)

	if err := ansibleRunnerCmd.Wait(); err != nil {
		return fmt.Errorf("error executing ansible-runner: err=%w", err)
	}

	r.processStatus()

	return nil
}

// handleJobEvent is the handler function invoked each time a job_event file is
// written to the job_events directory.
func (r *Runner) handleJobEvent(event notify.EventInfo) {
	if strings.HasSuffix(event.Path(), ".json") && !strings.Contains(event.Path(), "partial") {
		data, err := os.ReadFile(event.Path())
		if err != nil {
			log.Errorf("cannot read file: file=%v error=%v", event.Path(), err)
			return
		}

		// Unmarshal the ansibleEvent data into an untyped map instead of a
		// strictly typed structure. Using a strictly typed struct has the
		// unintentional side effect of discarding any fields from the
		// ansible-running ansibleEvent JSON that are not explicitly named in a
		// struct. This allows the fields that are immaterial to the worker to
		// still be included in the data structure.
		var ansibleEvent map[string]any
		if err := json.Unmarshal(data, &ansibleEvent); err != nil {
			log.Errorf("cannot unmarshal data: data=%v error=%v", data, err)
			return
		}

		// log the full event
		log.Debugf("received job event: %v", prettyJson(data))

		eventData, ok := ansibleEvent["event_data"]
		if !ok {
			eventData = map[string]any{}
		}
		if _, has := eventData.(map[string]any)["crc_dispatcher_correlation_id"]; !has {
			eventData.(map[string]any)["crc_dispatcher_correlation_id"] = r.ID
		}
		eventData.(map[string]any)["crc_message_version"] = 1
		ansibleEvent["event_data"] = eventData

		// "counter" is a required field according to playbook-dispatcher's
		// openapi schema. Messages without a "counter" field are rejected as
		// invalid by the server. The same is true for "start_line" and
		// "end_line".
		// https://github.com/RedHatInsights/playbook-dispatcher/blob/22853a47c5bb85c94fdb2a645fef02758247d4ae/schema/playbookRunResponse.message.yaml#L58-L63
		if _, has := ansibleEvent["counter"]; !has {
			ansibleEvent["counter"] = -1
		}
		if _, has := ansibleEvent["start_line"]; !has {
			ansibleEvent["start_line"] = 0
		}
		if _, has := ansibleEvent["end_line"]; !has {
			ansibleEvent["end_line"] = 0
		}

		modifiedData, err := json.Marshal(ansibleEvent)
		if err != nil {
			log.Errorf("cannot marshal JSON: err=%v", err)
			return
		}

		filteredModifiedData, err := filterJobEvent(modifiedData)

		if err != nil {
			// problem filtering, return original event
			log.Errorf("error filtering job event: err=%v", err)
			log.Info("sending unfiltered job event...")
			filteredModifiedData = modifiedData
		}

		r.Events <- filteredModifiedData
		log.Debugf("event sent: event=%v", prettyJson(filteredModifiedData))
	}
}

// processStatus reads the status file generated by ansible-runner
func (r *Runner) processStatus() {
	data, err := os.ReadFile(r.statusFilePath)
	if err != nil {
		log.Errorf("failed to read status file: err=%v", err)
		return
	}

	status := string(data)
	log.Infof("run complete: status=%v", status)

	if status == "failed" {
		// publish an "executor_on_failed" event to signal
		// cloud connector that a run has failed.
		statusFailedError := errors.New("playbook run failed")
		log.Error(statusFailedError)
		event := GenerateExecutorOnFailedEvent(r.ID, "UNDEFINED_ERROR", statusFailedError, uuid.New)

		data, err := json.Marshal(event)
		if err != nil {
			log.Errorf("cannot marshal JSON: err=%v", err)
			return
		}
		r.Events <- json.RawMessage(data)
	}

	r.Status = status
}

// end signals watchJobEvents to stop watching the job_events path and closes the Events channel
func (r *Runner) end() {
	close(r.stopJobEventsWatch)
	close(r.Events)
}

// watchJobEvents will set up a watch on r.jobEventsPath.
// Each time an event occurs, the handler function is invoked on the event.
// To stop the watch routine, send a value on the stop channel.
func (r *Runner) watchJobEvents() {
	watchedEvents := make(chan notify.EventInfo, 1)
	defer close(watchedEvents)

	if err := notify.Watch(r.jobEventsPath, watchedEvents, notify.InMovedTo); err != nil {
		log.Errorf("cannot watch path for events: path=%v err=%v", r.jobEventsPath, err)
		return
	}
	defer notify.Stop(watchedEvents)

	log.Tracef("start watching for job events: path=%v", r.jobEventsPath)
	defer log.Tracef("stop watching for job events: path=%v", r.jobEventsPath)

	for {
		select {
		case <-r.stopJobEventsWatch:
			return
		case event := <-watchedEvents:
			r.handleJobEvent(event)
		}
	}
}

// GenerateExecutorOnStartEvent creates a special executor_on_start event
// to inform Insights that the Ansible job is beginning.
func GenerateExecutorOnStartEvent(
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

// GenerateExecutorOnFailedEvent creates a special executor_on_failed event
// to inform Insights that the Ansible job failed to run.
func GenerateExecutorOnFailedEvent(
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

// filterJobEvent filters the Ansible job event based on
// a built-in schema of known necessary data
func filterJobEvent(jobEventData []byte) ([]byte, error) {
	// filter the event by narrowing to the playbook-dispatcher types
	var filteredEvent PlaybookRunResponseMessageEventsElem
	if err := json.Unmarshal(jobEventData, &filteredEvent); err != nil {
		return nil, err
	}

	filteredData, err := json.Marshal(filteredEvent)
	if err != nil {
		return nil, err
	}

	return filteredData, nil
}

// prettyJson pretty-prints a given JSON byte array
func prettyJson(jsonBytes []byte) string {
	var jsonObject map[string]any
	if err := json.Unmarshal(jsonBytes, &jsonObject); err != nil {
		log.Errorf("cannot unmarshal JSON: err=%v", err)
		return ""
	}
	pretty, err := json.MarshalIndent(jsonObject, "", "\t")
	if err != nil {
		log.Errorf("cannot marshal JSON: err=%v", err)
		return fmt.Sprintf("%v", jsonObject)
	}
	return string(pretty)
}
