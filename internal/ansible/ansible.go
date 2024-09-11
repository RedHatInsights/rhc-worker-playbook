package ansible

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~spc/go-log"
	"github.com/google/uuid"
	"github.com/redhatinsights/rhc-worker-playbook/internal/constants"
	"github.com/redhatinsights/rhc-worker-playbook/internal/exec"
	"github.com/rjeczalik/notify"
)

// Event represents data from an Ansible Runner event.
type Event struct {
	Event       string `json:"event"`
	UUID        string `json:"uuid"`
	Counter     int    `json:"counter"`
	Stdout      string `json:"stdout"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	RunnerIdent string `json:"runner_ident"`
	Created     string `json:"created"`
	EventData   struct {
		CRCDispatcherCorrelationID string `json:"crc_dispatcher_correlation_id"`
		CRCDispatcherErrorCode     string `json:"crc_dispatcher_error_code"`
		CRCDispatcherErrorDetails  string `json:"crc_dispatcher_error_details"`
	} `json:"event_data"`
}

// RunPlaybook creates an ansible-runner job to run the provided playbook. The
// function returns a channel over which the caller can receive ansible-runner
// events as they happen. The channel is closed when the job completes.
func RunPlaybook(id string, playbook []byte, correlationID string) (chan Event, error) {
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

	events := make(chan Event)

	// publish an "executor_on_start" event to signal cloud connector that a run
	// event has started. This is run on a goroutine in case the events
	// channel doesn't have a receiver connected yet to avoid blocking the
	// continuation of this function.
	go func() {
		events <- Event{
			Event:     "executor_on_start",
			UUID:      uuid.New().String(),
			Counter:   -1,
			Stdout:    "",
			StartLine: 0,
			EndLine:   0,
			EventData: struct {
				CRCDispatcherCorrelationID string `json:"crc_dispatcher_correlation_id"`
				CRCDispatcherErrorCode     string `json:"crc_dispatcher_error_code"`
				CRCDispatcherErrorDetails  string `json:"crc_dispatcher_error_details"`
			}{
				CRCDispatcherCorrelationID: correlationID,
			},
		}
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

						var event Event
						if err := json.Unmarshal(data, &event); err != nil {
							log.Errorf("cannot unmarshal data: data=%v error=%v", data, err)
							continue
						}

						events <- event
						log.Infof("event sent: event=%v", event)
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
			log.Tracef("start goroutine watching status file: %v", jobEventsDir)
			defer log.Tracef("stop goroutine watching status file: %v", jobEventsDir)

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
						events <- Event{
							Event:     "executor_on_failed",
							UUID:      uuid.New().String(),
							Counter:   -1,
							StartLine: 0,
							EndLine:   0,
							EventData: struct {
								CRCDispatcherCorrelationID string `json:"crc_dispatcher_correlation_id"`
								CRCDispatcherErrorCode     string `json:"crc_dispatcher_error_code"`
								CRCDispatcherErrorDetails  string `json:"crc_dispatcher_error_details"`
							}{
								CRCDispatcherCorrelationID: correlationID,
								CRCDispatcherErrorCode:     "UNDEFINED_ERROR",
							},
						}
					case "successful":
					default:
						log.Errorf("unsupported status case: %v", string(data))
					}
				}
			}
		}(statusFileChan)

		// Block the remainder of the routine until the process exits. When it
		// does, clean up.
		err := exec.WaitProcess(pid, func(pid int, state *os.ProcessState) {
			log.Debugf("process stopped: pid=%v runner_ident=%v", pid, id)
			if err := os.Remove(filepath.Join(constants.StateDir, id+".yaml")); err != nil {
				log.Errorf(
					"cannot remove file: file=%v error=%v",
					filepath.Join(constants.StateDir, id+".yaml"),
					err,
				)
			}
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
