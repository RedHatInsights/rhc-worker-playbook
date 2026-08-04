package main

import (
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
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
			slog.SetLogLoggerLevel(slog.LevelDebug)
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

func TestYAMLCustomUnmarshaler(t *testing.T) {
	t.Run("Custom unmarshaler can parse 1.1 booleans", func(t *testing.T) {

		testTruePlaybooks := [][]byte{
			[]byte(`- name: y Playbook
  hosts: localhost
  become: y
  vars: {}
  tasks: []
`),

			[]byte(`- name: Y Playbook
  hosts: localhost
  become: Y
  vars: {}
  tasks: []
`),

			[]byte(`- name: yes Playbook
  hosts: localhost
  become: yes
  vars: {}
  tasks: []
`),
			[]byte(`- name: Yes Playbook
  hosts: localhost
  become: Yes
  vars: {}
  tasks: []
`),

			[]byte(`- name: YES Playbook
  hosts: localhost
  become: YES
  vars: {}
  tasks: []
`),

			[]byte(`- name: on Playbook
  hosts: localhost
  become: on
  vars: {}
  tasks: []
`),

			[]byte(`- name: On Playbook
  hosts: localhost
  become: On
  vars: {}
  tasks: []
`),

			[]byte(`- name: ON Playbook
  hosts: localhost
  become: ON
  vars: {}
  tasks: []
`),

			[]byte(`- name: true Playbook
  hosts: localhost
  become: true
  vars: {}
  tasks: []
`),

			[]byte(`- name: True Playbook
  hosts: localhost
  become: True
  vars: {}
  tasks: []
`),

			[]byte(`- name: TRUE Playbook
  hosts: localhost
  become: TRUE
  vars: {}
  tasks: []
`)}

		testFalsePlaybooks := [][]byte{
			[]byte(`- name: n Playbook
  hosts: localhost
  become: n
  vars: {}
  tasks: []
`),

			[]byte(`- name: N Playbook
  hosts: localhost
  become: N
  vars: {}
  tasks: []
`),

			[]byte(`- name: no Playbook
  hosts: localhost
  become: no
  vars: {}
  tasks: []
`),

			[]byte(`- name: No Playbook
  hosts: localhost
  become: No
  vars: {}
  tasks: []
`),

			[]byte(`- name: NO Playbook
  hosts: localhost
  become: NO
  vars: {}
  tasks: []
`),

			[]byte(`- name: off Playbook
  hosts: localhost
  become: off
  vars: {}
  tasks: []
`),

			[]byte(`- name: Off Playbook
  hosts: localhost
  become: Off
  vars: {}
  tasks: []
`),

			[]byte(`- name: OFF Playbook
  hosts: localhost
  become: OFF
  vars: {}
  tasks: []
`),

			[]byte(`- name: false Playbook
  hosts: localhost
  become: false
  vars: {}
  tasks: []
`),

			[]byte(`- name: False Playbook
  hosts: localhost
  become: False
  vars: {}
  tasks: []
`),

			[]byte(`- name: FALSE Playbook
  hosts: localhost
  become: FALSE
  vars: {}
  tasks: []
`)}

		testErrorPlaybook := []byte(`- name: Error Playbook
  hosts: localhost
  become: death, destroyer of worlds
  vars: {}
  tasks: []
`)

		testingMatrix := []struct {
			playbooks [][]byte
			want      bool
		}{
			{
				playbooks: testTruePlaybooks,
				want:      true,
			}, {
				playbooks: testFalsePlaybooks,
				want:      false,
			},
		}

		for _, testValues := range testingMatrix {
			for _, testPb := range testValues.playbooks {
				var pbs []Playbook
				if err := yaml.UnmarshalWithOptions(testPb, &pbs); err != nil {
					t.Errorf("%v", err)
				}

				got := pbs[0].Become
				if got != testValues.want {
					t.Errorf("\ngot:\n%v\nwant:\n%v", got, testValues.want)
				}
			}
		}

		// finally, make sure it errors on invalid values
		var playbooks []Playbook
		err := yaml.UnmarshalWithOptions(testErrorPlaybook, &playbooks)

		if err == nil {
			t.Errorf("there should have been a YAML parsing error: %v", testErrorPlaybook)
		}

		if !strings.Contains(err.Error(), "unable to parse boolean") {
			t.Errorf("unknown error: %v", err)
		}

	})

	t.Run("Marshaler serializes yaml with become: true or become: false", func(t *testing.T) {
		truePlaybook := []Playbook{
			{
				Name:   "True Playbook",
				Hosts:  "localhost",
				Become: true,
				Vars:   map[string]any{},
				Tasks:  []yaml.MapSlice{},
			},
		}

		falsePlaybook := []Playbook{
			{
				Name:   "False Playbook",
				Hosts:  "localhost",
				Become: false,
				Vars:   map[string]any{},
				Tasks:  []yaml.MapSlice{},
			},
		}

		playbookData, err := yaml.MarshalWithOptions(truePlaybook, yaml.IndentSequence(false))
		if err != nil {
			t.Errorf("%v", err)
		}
		if !strings.Contains(string(playbookData), "become: true") {
			t.Errorf("incorrect marshaling: %v", playbookData)
		}

		playbookData, err = yaml.MarshalWithOptions(falsePlaybook, yaml.IndentSequence(false))
		if err != nil {
			t.Errorf("%v", err)
		}
		if !strings.Contains(string(playbookData), "become: false") {
			t.Errorf("incorrect marshaling: %v", playbookData)
		}

	})
}
