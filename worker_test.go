package main

import (
	"os"
	"testing"

	"git.sr.ht/~spc/go-log"
	"github.com/google/go-cmp/cmp"
)

func readFile(t *testing.T, file string) []byte {
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("cannot read file %v: %v", file, err)
	}
	return data
}

func TestVerifyPlaybook(t *testing.T) {
	if os.Getenv("USER") != "root" {
		t.Skip("TestVerifyPlaybook requires insights-client; therefore it must run as root")
	}

	tests := []struct {
		description string
		input       struct {
			playbook []byte
			GPGCheck bool
		}
		want []byte
	}{
		{
			description: "insights_remove.yml",
			input: struct {
				playbook []byte
				GPGCheck bool
			}{
				playbook: readFile(t, "./testdata/insights_remove.yml"),
				GPGCheck: true,
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
			log.SetLevel(log.LevelDebug)
			got, err := verifyPlaybook(test.input.playbook, test.input.GPGCheck)
			if err != nil {
				t.Fatal(err)
			}

			if !cmp.Equal(got, test.want) {
				t.Errorf("\ngot:\n%v\nwant:\n%v", string(got), string(test.want))
			}
		})
	}
}
