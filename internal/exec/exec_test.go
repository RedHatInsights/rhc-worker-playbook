package exec

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestStartProcess(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			file string
			args []string
			env  []string
		}
	}{
		{
			input: struct {
				file string
				args []string
				env  []string
			}{
				file: "/usr/bin/sleep",
				args: []string{"1"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			err := StartProcess(test.input.file, test.input.args, test.input.env, nil)
			if err != nil {
				t.Fatalf("unable to start process: %v", err)
			}
		})
	}
}

func TestStopProcess(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			file string
			args []string
			env  []string
		}
	}{
		{
			input: struct {
				file string
				args []string
				env  []string
			}{
				file: "/usr/bin/sleep",
				args: []string{"3"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			err := StartProcess(
				test.input.file,
				test.input.args,
				test.input.env,
				func(pid int, stdout, stderr io.ReadCloser) {
					if err := StopProcess(pid); err != nil {
						t.Fatalf("unable to stop process: %v", err)
					}
				},
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWaitProcess(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			file string
			args []string
			env  []string
		}
	}{
		{
			input: struct {
				file string
				args []string
				env  []string
			}{
				file: "/usr/bin/sleep",
				args: []string{"3"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {

			err := StartProcess(
				test.input.file,
				test.input.args,
				test.input.env,
				func(startPid int, stdout, stderr io.ReadCloser) {
					if err := WaitProcess(startPid, func(stopPid int, state *os.ProcessState) {
						if startPid != stopPid {
							t.Fatalf("%v != %v", startPid, stopPid)
						}
						if !state.Exited() {
							t.Fatalf("unexpected process exit state")
						}
					}); err != nil {
						t.Fatalf("unable to wait for process with pid %v: %v", startPid, err)
					}
				},
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunProcess(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			file  string
			args  []string
			env   []string
			stdin io.Reader
		}
		want struct {
			stdout []byte
			stderr []byte
			code   int
		}
	}{
		{
			input: struct {
				file  string
				args  []string
				env   []string
				stdin io.Reader
			}{
				file: "/usr/bin/echo",
				args: []string{"hello"},
			},
			want: struct {
				stdout []byte
				stderr []byte
				code   int
			}{
				stdout: []byte("hello\n"),
			},
		},
		{
			input: struct {
				file  string
				args  []string
				env   []string
				stdin io.Reader
			}{
				file:  "/usr/bin/cat",
				stdin: bytes.NewReader([]byte("hello")),
			},
			want: struct {
				stdout []byte
				stderr []byte
				code   int
			}{
				stdout: []byte("hello"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			gotout, goterr, gotcode, err := RunProcess(
				test.input.file,
				test.input.args,
				test.input.env,
				test.input.stdin,
			)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !cmp.Equal(gotout, test.want.stdout, cmpopts.EquateEmpty()) {
				t.Errorf("%v != %v", gotout, test.want.stdout)
			}

			if !cmp.Equal(goterr, test.want.stderr, cmpopts.EquateEmpty()) {
				t.Errorf("%v != %v", goterr, test.want.stderr)
			}

			if gotcode != test.want.code {
				t.Errorf("%v != %v", gotcode, test.want.code)
			}
		})
	}
}
