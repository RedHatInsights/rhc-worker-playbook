package ansible

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/redhatinsights/rhc-worker-playbook/internal/constants"
	"github.com/rjeczalik/notify"
	"github.com/subpop/go-log"
)

// Runner maintains the state of a playbook run during execution.
type Runner struct {
	// correlationId is the identity of the job. It is used in file paths and event
	// metadata.
	correlationId string

	// events contain runner events, marshaled as raw JSON. Receive values from
	// this channel to receive the current state of the run.
	events chan json.RawMessage

	// Status is the final status of the run. It will be empty if the job has
	// not completed.
	Status string

	playbookPath   string
	jobEventsPath  string
	statusFilePath string

	stopJobEventsWatch chan struct{}
}

// NewRunner creates a new Runner, uniquely identified by ID.
func NewRunner(correlationId string, events chan json.RawMessage) *Runner {
	return &Runner{
		events:        events,
		correlationId: correlationId,
		playbookPath:  filepath.Join(constants.StateDir, correlationId+".yaml"),
		jobEventsPath: filepath.Join(
			constants.PrivateDataDir, "artifacts", correlationId, "job_events",
		),
		statusFilePath: filepath.Join(
			constants.PrivateDataDir, "artifacts", correlationId, "status",
		),
		stopJobEventsWatch: make(chan struct{}),
	}
}

// Run begins running the provided playbook, using the given ID as the run
// identity. It returns after ansible-runner completes the playbook run.
// Events will be sent to the runner's events channel. When the channel closes,
// the run is complete.
func (r *Runner) Run(playbook []byte) error {
	defer close(r.stopJobEventsWatch)

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

	ansibleRunnerCmd := exec.Command(
		"/usr/bin/python3",
		"-m",
		"ansible_runner",
		"run",
		"--ident",
		r.correlationId,
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

	defer func() {
		log.Infof("run complete: status=%v", r.Status)
	}()

	if err := r.processStatus(); err != nil {
		return err
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
			eventData.(map[string]any)["crc_dispatcher_correlation_id"] = r.correlationId
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

		r.events <- filteredModifiedData
		log.Debugf("event sent: event=%v", prettyJson(filteredModifiedData))
	}
}

// processStatus reads the status file generated by ansible-runner
func (r *Runner) processStatus() error {
	data, err := os.ReadFile(r.statusFilePath)
	if err != nil {
		return fmt.Errorf("failed to read status file: err=%v", err)
	}

	r.Status = string(data)

	if r.Status == "failed" {
		// publish an "executor_on_failed" event to signal
		// cloud connector that a run has failed.
		statusFailedError := errors.New("playbook run failed")
		return statusFailedError
	}

	return nil
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
