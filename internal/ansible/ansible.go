package ansible

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/google/uuid"
	"github.com/redhatinsights/rhc-worker-playbook/internal/constants"
	"github.com/redhatinsights/rhc-worker-playbook/internal/exec"
	"github.com/rjeczalik/notify"
)

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
	if err := os.WriteFile(playbookPath, playbook, 0600); err != nil {
		return fmt.Errorf("cannot write playbook file: path=%v err=%w", playbookPath, err)
	}

	// precreate the job_events directory so that we can watch for when events
	// get written to it.
	jobEventsPath := filepath.Join(r.privateDataDir, "artifacts", r.ID, "job_events")
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
		event := map[string]interface{}{
			"event":      "executor_on_start",
			"uuid":       uuid.New().String(),
			"counter":    -1,
			"stdout":     "",
			"start_line": 0,
			"end_line":   0,
			"event_data": map[string]interface{}{
				"crc_dispatcher_correlation_id": r.ID,
			},
		}
		data, err := json.Marshal(event)
		if err != nil {
			log.Errorf("cannot marshal json: err=%v", err)
			return
		}
		r.Events <- data
	}()

	err = exec.StartProcess(
		"/usr/bin/python3",
		[]string{
			"-m",
			"ansible_runner",
			"start",
			"--ident",
			r.ID,
			"--playbook",
			playbookPath,
			r.privateDataDir,
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
		func(pid int, stdout, stderr io.ReadCloser) {
			log.Infof("run started: pid=%v", pid)
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
		var ansibleEvent map[string]interface{}
		if err := json.Unmarshal(data, &ansibleEvent); err != nil {
			log.Errorf("cannot unmarshal data: data=%v error=%v", data, err)
			return
		}
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
			log.Errorf("cannot marshal JSON: err=%v", err)
			return
		}

		r.Events <- modifiedData
		log.Debugf("event sent: event=%v", ansibleEvent)
	}
}

// handleStatusFileEvent is the handler function invoked when the status file is
// written to.
func (r *Runner) handleStatusFileEvent(event notify.EventInfo) {
	data, err := os.ReadFile(event.Path())
	if err != nil {
		log.Errorf("failed to read status file: err=%v", err)
		return
	}

	status := string(data)
	log.Infof("run complete: status=%v", status)

	if status == "failed" {
		// publish an "executor_on_failed" event to signal
		// cloud connector that a run has failed.
		event := map[string]interface{}{
			"event":      "executor_on_failed",
			"uuid":       uuid.New().String(),
			"counter":    -1,
			"start_line": 0,
			"end_line":   0,
			"event_data": map[string]interface{}{
				"crc_dispatcher_correlation_id": r.ID,
				"crc_dispatcher_error_code":     "UNDEFINED_ERROR",
			},
		}

		data, err := json.Marshal(event)
		if err != nil {
			log.Errorf("cannot marshal JSON: err=%v", err)
			return
		}
		r.Events <- json.RawMessage(data)
	}

	r.Status = status

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
func (r Runner) watch(
	path string,
	handler func(event notify.EventInfo),
	stop chan struct{},
	timeout <-chan time.Time,
	events ...notify.Event,
) {
	watchedEvents := make(chan notify.EventInfo, 1)
	defer close(watchedEvents)

	if err := notify.Watch(path, watchedEvents, events...); err != nil {
		log.Errorf("cannot watch path for events: path=%v err=%v", path, err)
		return
	}
	defer notify.Stop(watchedEvents)

	log.Tracef("start watching for events: path=%v events=%v", path, events)
	defer log.Tracef("stop watching for events: path=%v", path)
	for {
		select {
		case <-timeout:
			log.Infof("timeout elapsed watching for events: path=%v", path)
			return
		case <-stop:
			return
		case event := <-watchedEvents:
			handler(event)
		}
	}
}
