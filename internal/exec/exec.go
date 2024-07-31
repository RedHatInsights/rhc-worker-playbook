package exec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type ProcessStartedFunc func(pid int, stdout, stderr io.ReadCloser)
type ProcessStoppedFunc func(pid int, state *os.ProcessState)

// StartProcess executes file, setting up the environment using the provided env
// values. If the function parameter started is not nil, it is invoked on a
// goroutine after the process has been started.
func StartProcess(
	file string,
	args []string,
	env []string,
	started ProcessStartedFunc,
) error {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return fmt.Errorf("cannot find file: %v", err)
	}

	cmd := exec.Command(file, args...)
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("cannot connect to stdout: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("cannot connect to stderr: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot start worker: %v: %v", file, err)

	}

	if started != nil {
		go started(cmd.Process.Pid, stdout, stderr)
	}

	return nil
}

// WaitProcess finds a process with the given pid and waits for it to exit.
// If the function parameter stopped is not nil, it is invoked on a goroutine
// when the process exits.
func WaitProcess(pid int, stopped ProcessStoppedFunc) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("cannot find process with pid: %v", err)
	}

	state, err := process.Wait()
	if err != nil {
		return fmt.Errorf("process %v exited with error: %v", process.Pid, err)
	}

	if stopped != nil {
		go stopped(process.Pid, state)
	}

	return nil
}

// StopProcess finds a process with the given pid and kills it.
func StopProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("cannot find process with pid: %v", err)
	}

	if err := process.Kill(); err != nil {
		return fmt.Errorf("cannot stop process %v", err)
	}

	return nil
}

// RunProcess executes file, setting up environment using the provided env
// values. It waits for the process to finish and returns the stdout, stderr and
// return code.
func RunProcess(
	file string,
	args []string,
	env []string,
	stdin io.Reader,
) (stdout []byte, stderr []byte, code int, err error) {
	cmd := exec.Command(file, args...)
	cmd.Env = env

	cmd.Stdin = stdin

	outb := new(bytes.Buffer)
	cmd.Stdout = outb

	errb := new(bytes.Buffer)
	cmd.Stderr = errb

	err = cmd.Run()

	stdout = outb.Bytes()
	stderr = errb.Bytes()
	code = cmd.ProcessState.ExitCode()

	return
}
