package main

import (
	"log/slog"
	"os"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/rhc-worker-playbook/internal/log"
)

func readFile(t *testing.T, file string) []byte {
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("cannot read file %v: %v", file, err)
	}
	return data
}

func TestVerifyPlaybook(t *testing.T) {
	_, err := exec.LookPath("/usr/libexec/rhc-playbook-verifier")
	if err != nil {
		t.Skip("rhc-playbook-verifier is not installed")
	}

	tests := []struct {
		description string
		input       struct {
			playbook []byte
		}
		want []byte
	}{
		{
			description: "insights_remove.yml",
			input: struct {
				playbook []byte
			}{
				playbook: readFile(t, "./testdata/insights_remove.yml"),
			},
			want: []byte(`- name: Insights Disable
  hosts: localhost
  become: yes
  vars:
    insights_signature_exclude: /hosts,/vars/insights_signature
  tasks:
  - name: Disable the insights-client
    command: insights-client --disable-schedule
`),
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			log.SetLevel(slog.LevelDebug)
			got, err := verifyPlaybook(test.input.playbook)
			if err != nil {
				t.Fatal(err)
			}

			if !cmp.Equal(got, test.want) {
				t.Errorf("\ngot:\n%v\nwant:\n%v", string(got), string(test.want))
			}
		})
	}
}
