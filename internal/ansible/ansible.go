package ansible

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/redhatinsights/rhc-worker-playbook/internal/constants"
	"github.com/redhatinsights/rhc-worker-playbook/internal/exec"
	"github.com/rjeczalik/notify"
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

	privateDataDir      string
	stopJobEventsWatch  chan struct{}
	stopStatusFileWatch chan struct{}
	timeout             time.Duration
}

// createUuidFunc is a function that returns a UUID, typically uuid.New(),
// used as a function parameter to decouple uuid generation from function logic
type createUuidFunc func() uuid.UUID

// NewRunner creates a new Runner, uniquely identified by ID.
func NewRunner(ID string, timeout time.Duration) *Runner {
	return &Runner{
		Events:              make(chan json.RawMessage),
		privateDataDir:      filepath.Join(constants.StateDir, "runs"),
		ID:                  ID,
		stopJobEventsWatch:  make(chan struct{}),
		stopStatusFileWatch: make(chan struct{}),
		timeout:             timeout,
	}
}

// Run begins running the provided playbook, using the given ID as the run
// identity. It returns immediately after starting the ansible_runner process.
// Events will be sent to the runner's Events channel. When the channel closes,
// the run is complete.
func (r *Runner) Run(playbook []byte) error {
	// write playbook to the filesystem
	playbookPath := filepath.Join(constants.StateDir, r.ID+".yaml")
	slog.Info("writing playbook to file:", "path", playbookPath)

	if err := os.WriteFile(playbookPath, playbook, 0600); err != nil {
		return fmt.Errorf("cannot write playbook file: path=%v err=%w", playbookPath, err)
	}

	// precreate the job_events directory so that we can watch for when events
	// get written to it.
	jobEventsPath := filepath.Join(r.privateDataDir, "artifacts", r.ID, "job_events")
	slog.Info("creating job_events directory:", "path", jobEventsPath)

	if err := os.MkdirAll(jobEventsPath, 0755); err != nil {
		return fmt.Errorf(
			"cannot create job_events directory: directory=%v err=%w",
			jobEventsPath,
			err,
		)
	}

	// precreate the status file so that we can watch for when it gets written
	// to.
	statusFilePath := filepath.Join(r.privateDataDir, "artifacts", r.ID, "status")
	slog.Info("writing status file:", "path", statusFilePath)

	status, err := os.Create(statusFilePath)
	if err != nil {
		return fmt.Errorf("cannot create status file: file=%v err=%w", statusFilePath, err)
	}
	_ = status.Close()

	// start a goroutine to watch for event files being written to the
	// job_events directory. When a relevant event file is detected, it gets
	// marshaled into JSON and sent to the events channel.
	go r.watch(jobEventsPath, r.handleJobEvent, r.stopJobEventsWatch, nil, notify.InMovedTo)

	// start a goroutine that watches for the "status" file. When it gets
	// written to, its contents are read, and if the status is "failed", a final
	// "failed" event is written to the events channel. No action is taken if
	// the status is "successful".
	go r.watch(
		statusFilePath,
		r.handleStatusFileEvent,
		r.stopStatusFileWatch,
		time.After(r.timeout),
		notify.InCloseWrite,
	)

	// publish an "executor_on_start" event to signal cloud connector that a run
	// event has started. This is run on a goroutine in case the events
	// channel doesn't have a receiver connected yet to avoid blocking the
	// continuation of this function.
	go func() {
		event := GenerateExecutorOnStartEvent(r.ID, uuid.New)
		data, err := json.Marshal(event)
		if err != nil {
			slog.Error("cannot marshal json:", "err", err)
			return
		}
		r.Events <- data
	}()

	// typically /var/lib/rhc-worker-playbook/ansible-home
	// created automatically by ansible_runner
	ansibleHomePath := filepath.Join(constants.StateDir, "ansible-home")

	// typically /var/lib/rhc-worker-playbook/ansible-home/remote-tmp
	// created automatically by ansible_runner
	ansibleRemoteTmpPath := filepath.Join(ansibleHomePath, "remote-tmp")

	args := []string{
		"-m",
		"ansible_runner",
		"start",
		"--ident",
		r.ID,
		"--playbook",
		playbookPath,
		r.privateDataDir,
	}
	env := []string{
		"PATH=/sbin:/bin:/usr/sbin:/usr/bin",
		"PYTHONPATH=" + filepath.Join(constants.LibDir, "rhc-worker-playbook"),
		"PYTHONDONTWRITEBYTECODE=1",
		"ANSIBLE_HOME=" + ansibleHomePath,
		"ANSIBLE_REMOTE_TMP=" + ansibleRemoteTmpPath,
		"ANSIBLE_COLLECTIONS_PATH=" + filepath.Join(
			constants.DataDir,
			"rhc-worker-playbook",
			"ansible",
			"collections",
			"ansible_collections",
		),
	}

	slog.Info("launching python3 (ansible-runner) subprocess")
	slog.Debug("launching with parameters:",
		"args", args,
		"env", env,
	)

	err = exec.StartProcess(
		"/usr/bin/python3",
		args,
		env,
		func(pid int, stdout, stderr io.ReadCloser) {
			slog.Info("run started:", "pid", pid)
		},
	)

	if err != nil {
		return fmt.Errorf("cannot start process: err=%w", err)
	}

	return nil
}

// handleJobEvent is the handler function invoked each time a job_event file is
// written to the job_events directory.
func (r *Runner) handleJobEvent(event notify.EventInfo) {
	eventPath := event.Path()
	if strings.HasSuffix(eventPath, ".json") && !strings.Contains(eventPath, "partial") {
		data, err := os.ReadFile(eventPath)
		if err != nil {
			slog.Error("cannot read file:",
				"file", eventPath,
				"error", err)
			return
		}

		// Unmarshal the ansibleEvent data into an untyped map instead of a
		// strictly typed structure. Using a strictly typed struct has the
		// unintentional side effect of discarding any fields from the
		// ansible-running ansibleEvent JSON that are not explicitly named in a
		// struct. This allows the fields that are immaterial to the worker to
		// still be included in the data structure.
		var ansibleEvent map[string]interface{}
		if err := json.Unmarshal(data, &ansibleEvent); err != nil {
			slog.Error("cannot unmarshal data:",
				"data", data,
				"error", err)
			return
		}

		// log the full event
		slog.Debug("received job event:", "event", prettyJson(data))

		eventData, ok := ansibleEvent["event_data"]
		if !ok {
			eventData = map[string]interface{}{}
		}
		if _, has := eventData.(map[string]interface{})["crc_dispatcher_correlation_id"]; !has {
			eventData.(map[string]interface{})["crc_dispatcher_correlation_id"] = r.ID
		}
		eventData.(map[string]interface{})["crc_message_version"] = 1
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
			slog.Error("cannot marshal JSON:", "err", err)
			return
		}

		filteredModifiedData, err := filterJobEvent(modifiedData)

		if err != nil {
			// problem filtering, return original event
			slog.Error("error filtering job event:", "err", err)
			slog.Info("sending unfiltered job event...")
			filteredModifiedData = modifiedData
		}

		r.Events <- filteredModifiedData
		slog.Debug("sent job event:", "event", prettyJson(filteredModifiedData))
	}
}

// handleStatusFileEvent is the handler function invoked when the status file is
// written to.
func (r *Runner) handleStatusFileEvent(event notify.EventInfo) {
	// TODO [RHINENG-22379]: what happens to the playbook process when the status file is unreadable?
	//	and/or what happens when this function returns without closing the channels?
	data, err := os.ReadFile(event.Path())
	if err != nil {
		slog.Error("failed to read status file:", "err", err)
		return
	}

	status := string(data)
	slog.Info("run complete:", "status", status)

	if status == "failed" {
		// publish an "executor_on_failed" event to signal
		// cloud connector that a run has failed.
		statusFailedError := errors.New("playbook run failed")
		slog.Error(statusFailedError.Error())
		event := GenerateExecutorOnFailedEvent(r.ID, "UNDEFINED_ERROR", statusFailedError, uuid.New)

		data, err := json.Marshal(event)
		if err != nil {
			slog.Error("cannot marshal JSON:", "err", err)
			return
		}
		r.Events <- json.RawMessage(data)
	}

	r.Status = status

	r.end()
}

// end monitoring playbook event output
func (r *Runner) end() {
	// Close the events channel, signalling to callers that the job is complete.
	close(r.Events)

	// Signal the watch routines to stop and clean up. This is done on a
	// goroutine to allow the executing handlers an opportunity to finish before
	// the watch is removed.
	go func() {
		r.stopJobEventsWatch <- struct{}{}
		r.stopStatusFileWatch <- struct{}{}
	}()
}

// watch will set up a watch on the file or directory specified by path. Each
// time an event occurs, the handler function is invoked on the event. To stop
// the watch routine, send a value on the stop channel.
func (r *Runner) watch(
	path string,
	handler func(event notify.EventInfo),
	stop chan struct{},
	timeout <-chan time.Time,
	events ...notify.Event,
) {
	watchedEvents := make(chan notify.EventInfo, 1)
	defer close(watchedEvents)

	if err := notify.Watch(path, watchedEvents, events...); err != nil {
		slog.Error("cannot watch path for events:",
			"path", path,
			"err", err,
		)
		return
	}
	defer notify.Stop(watchedEvents)

	slog.Info("start watching for events:",
		"path", path,
		"events", events,
	)
	defer slog.Info("stop watching for events:", "path", path)
	for {
		select {
		case <-timeout:
			slog.Info("timeout elapsed watching for events:", "path", path)
			// since the status file handler is not invoked, clean up channels here, otherwise it will upload forever
			r.end()
			return
		case <-stop:
			return
		case event := <-watchedEvents:
			handler(event)
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
		slog.Error("cannot unmarshal JSON:", "err", err)
		return ""
	}
	pretty, err := json.MarshalIndent(jsonObject, "", "\t")
	if err != nil {
		slog.Error("cannot marshal JSON:", "err", err)
		return fmt.Sprintf("%v", jsonObject)
	}
	return string(pretty)
}
