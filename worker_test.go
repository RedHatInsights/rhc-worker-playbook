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
		// this NEEDS to be on the system to properly test this functionality
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
			want: []byte(`# This playbook will take care of all steps required to disable
# Insights Client
- name: Insights Disable
  hosts: localhost
  become: yes
  vars:
    insights_signature_exclude: /hosts,/vars/insights_signature
    insights_signature: !!binary |
      TFMwdExTMUNSVWRKVGlCUVIxQWdVMGxIVGtGVVZWSkZMUzB0TFMwS1ZtVnljMmx2YmpvZ1IyNTFV
      RWNnZGpFS0NtbFJTV05DUVVGQ1EwRkJSMEpSU20xdGIwRmtRVUZ2U2tWTmRuYzFPRVFyYWpWd1Rr
      UlpTVkF2YVVwV2VWWnZkbE5LYTNwdUwweFNjMVpEVGtGWVQzY0tXa3QzYnpBM1RXMTFiVFJQZFdG
      c2MwVTFiakZHUTBSd0wyNW5URWhQYjBGT2JXZG9lV2RaVVdnclNYSk5SVFphTW5kNGRtUnVVMUU1
      Um5oaFZEQlJRUW8wYURoNVJHaEhXWGw0TVhwUlRYbGtRV3QzUmtwRmFsVjJabTVoVmpNM2RFazFR
      MVJtU1RacU5HaEhaREJpVFROclNIVk9hR3RSYzNWbmJIZFJNMlV5Q21WdlUzVkZjVkpZVDBwbkwz
      bEdVa05GYkRONGIzSllUSEoxU3psdFRIRlRTR05PZFZWSWJWVjBPWEpUY0ROalpsTjZUSHBHV2l0
      WFJWcDNXWGw1TmpNS1ZXSnBaVzAxUzFoUVpESk9ZbE5YVHpseFZpOU5kaTl1Y25WdVlXRnpRVkJW
      T0dWa2MxRjJTWFJGZVVzemJqSlNWMnhMWkhCbFEzUXplV3RsWjBFeGNRcFhkRXRJVWxVNU9XRmla
      Mkp6VUZoUVJ6WXdPV1ZOWkdGTlIxZEZlWHBuVmpsV2NYSkxTR3BhVjJacGNtRTNTVUptTDFOVGMy
      cHZSMkpRUmxWR015OXFDaTlCVFUxdlRHaHFWbEI0WjNaTFZXaFdTV2hwUVd4VmFuaFBNM0JKTVRK
      emVrOXRiMGxFUkdKb2FHeE5lRGd5YzBkRWFuSktPRFZVWlUweGNuQjRZbklLVmxWemFraFpaRzQx
      UW10amVISm5aWGhJWVN0SmVFMXFhbXBEVmt4WmNUTlFkV2s1Um05dlF6TmpNRFo1ZFRBME4xcERW
      R2RoWTFSbFVuTXljMWhWWWdvM1pWWkVaMHh0ZG1sRVNFdGhLMVp2UmxOa01YQklSRVEzY0RGSFJY
      bFlUMjV1YzI1a05GUktVblZuT0VVMWJUUm5aSEp3VVRSTGNrY3pUR3RXYXpKQkNsQldlaXR1UzFO
      VmJuWkVRbVJ3YTNoT05EZDBaM014TjNOcFlqUm9Sak5JVFhGRWNWRk5abFphZWtwck4ySlFlRFZ5
      WlZGU1FXMUtPVUZYTkcwMlVEa0tURFZuVlRGSWRIbHBaa05zZEV4cllWcHlRVkJtVG1KRmIycGxk
      VVEwZG5CcU9EZEVPR2gxT0dJd05EaDBkeTlaVlhjdmIzQTJjMk5wWkVaM2VHdFRlUXBYZVZCRVFV
      bDJOME4xU2xwc2JFYzVUR05KVVFvOWRYbENaUW90TFMwdExVVk9SQ0JRUjFBZ1UwbEhUa0ZVVlZK
      RkxTMHRMUzBL
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

	// initialize bool pointers for comparison
	tru := new(bool)
	*tru = true
	fal := new(bool)
	*fal = false

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
			want      *bool
		}{
			{
				playbooks: testTruePlaybooks,
				want:      tru,
			}, {
				playbooks: testFalsePlaybooks,
				want:      fal,
			},
		}

		for _, testValues := range testingMatrix {
			for _, testPb := range testValues.playbooks {
				var pbs []Play
				if err := yaml.UnmarshalWithOptions(testPb, &pbs); err != nil {
					t.Errorf("%v", err)
				}

				got := pbs[0].Become
				if *got != *testValues.want {
					t.Errorf("\ngot:\n%v\nwant:\n%v", got, testValues.want)
				}
			}
		}

		// finally, make sure it errors on invalid values
		var playbooks []Play
		err := yaml.UnmarshalWithOptions(testErrorPlaybook, &playbooks)

		if err == nil {
			t.Errorf("there should have been a YAML parsing error: %v", testErrorPlaybook)
		}

		if !strings.Contains(err.Error(), "unable to parse boolean") {
			t.Errorf("unknown error: %v", err)
		}

	})

	t.Run("Marshaler serializes yaml with become: true or become: false", func(t *testing.T) {
		truePlaybook := []Play{
			{
				Name:   "True Playbook",
				Hosts:  "localhost",
				Become: tru,
				Vars:   map[string]any{},
				Tasks:  []yaml.MapSlice{},
			},
		}

		falsePlaybook := []Play{
			{
				Name:   "False Playbook",
				Hosts:  "localhost",
				Become: fal,
				Vars:   map[string]any{},
				Tasks:  []yaml.MapSlice{},
			},
		}

		playbookData, err := yaml.MarshalWithOptions(truePlaybook, yaml.IndentSequence(false))
		if err != nil {
			t.Errorf("%v", err)
		}
		if !strings.Contains(string(playbookData), "become: true") {
			t.Errorf("incorrect marshaling: %v", string(playbookData))
		}

		playbookData, err = yaml.MarshalWithOptions(falsePlaybook, yaml.IndentSequence(false))
		if err != nil {
			t.Errorf("%v", err)
		}
		if !strings.Contains(string(playbookData), "become: false") {
			t.Errorf("incorrect marshaling: %v", string(playbookData))
		}

	})
}

func TestStripSignature(t *testing.T) {
	t.Run("stripSignature returns the playbook with insights_signature removed from vars", func(t *testing.T) {
		// this is the output of verifyPlaybook, complete playbook data
		playbook := []byte(`# This playbook will take care of all steps required to disable
# Insights Client
- name: Insights Disable
  hosts: localhost
  become: yes
  vars:
    insights_signature_exclude: /hosts,/vars/insights_signature
    insights_signature: !!binary |
      TFMwdExTMUNSVWRKVGlCUVIxQWdVMGxIVGtGVVZWSkZMUzB0TFMwS1ZtVnljMmx2YmpvZ1IyNTFV
      RWNnZGpFS0NtbFJTV05DUVVGQ1EwRkJSMEpSU20xdGIwRmtRVUZ2U2tWTmRuYzFPRVFyYWpWd1Rr
      UlpTVkF2YVVwV2VWWnZkbE5LYTNwdUwweFNjMVpEVGtGWVQzY0tXa3QzYnpBM1RXMTFiVFJQZFdG
      c2MwVTFiakZHUTBSd0wyNW5URWhQYjBGT2JXZG9lV2RaVVdnclNYSk5SVFphTW5kNGRtUnVVMUU1
      Um5oaFZEQlJRUW8wYURoNVJHaEhXWGw0TVhwUlRYbGtRV3QzUmtwRmFsVjJabTVoVmpNM2RFazFR
      MVJtU1RacU5HaEhaREJpVFROclNIVk9hR3RSYzNWbmJIZFJNMlV5Q21WdlUzVkZjVkpZVDBwbkwz
      bEdVa05GYkRONGIzSllUSEoxU3psdFRIRlRTR05PZFZWSWJWVjBPWEpUY0ROalpsTjZUSHBHV2l0
      WFJWcDNXWGw1TmpNS1ZXSnBaVzAxUzFoUVpESk9ZbE5YVHpseFZpOU5kaTl1Y25WdVlXRnpRVkJW
      T0dWa2MxRjJTWFJGZVVzemJqSlNWMnhMWkhCbFEzUXplV3RsWjBFeGNRcFhkRXRJVWxVNU9XRmla
      Mkp6VUZoUVJ6WXdPV1ZOWkdGTlIxZEZlWHBuVmpsV2NYSkxTR3BhVjJacGNtRTNTVUptTDFOVGMy
      cHZSMkpRUmxWR015OXFDaTlCVFUxdlRHaHFWbEI0WjNaTFZXaFdTV2hwUVd4VmFuaFBNM0JKTVRK
      emVrOXRiMGxFUkdKb2FHeE5lRGd5YzBkRWFuSktPRFZVWlUweGNuQjRZbklLVmxWemFraFpaRzQx
      UW10amVISm5aWGhJWVN0SmVFMXFhbXBEVmt4WmNUTlFkV2s1Um05dlF6TmpNRFo1ZFRBME4xcERW
      R2RoWTFSbFVuTXljMWhWWWdvM1pWWkVaMHh0ZG1sRVNFdGhLMVp2UmxOa01YQklSRVEzY0RGSFJY
      bFlUMjV1YzI1a05GUktVblZuT0VVMWJUUm5aSEp3VVRSTGNrY3pUR3RXYXpKQkNsQldlaXR1UzFO
      VmJuWkVRbVJ3YTNoT05EZDBaM014TjNOcFlqUm9Sak5JVFhGRWNWRk5abFphZWtwck4ySlFlRFZ5
      WlZGU1FXMUtPVUZYTkcwMlVEa0tURFZuVlRGSWRIbHBaa05zZEV4cllWcHlRVkJtVG1KRmIycGxk
      VVEwZG5CcU9EZEVPR2gxT0dJd05EaDBkeTlaVlhjdmIzQTJjMk5wWkVaM2VHdFRlUXBYZVZCRVFV
      bDJOME4xU2xwc2JFYzVUR05KVVFvOWRYbENaUW90TFMwdExVVk9SQ0JRUjFBZ1UwbEhUa0ZVVlZK
      RkxTMHRMUzBL
  tasks:
    - name: Disable the insights-client
      command: insights-client --disable-schedule
- name: ping
  hosts: localhost
  vars:
    insights_signature_exclude: /hosts,/vars/insights_signature
    insights_signature: !!binary |
      TFMwdExTMUNSVWRKVGlCUVIxQWdVMGxIVGtGVVZWSkZMUzB0TFMwS1ZtVnljMmx2YmpvZ1IyNTFV
      RWNnZGpFS0NtbFJTVlZCZDFWQldVaHBSM0ZqZG5jMU9FUXJhalZ3VGtGUmFrTXpRUzh6VVdwUVow
      MXZTM0JYZFZVeWNuaExWVkpJYTI5VVRHVkdTRmczVDFkVU1Ya0tlRzR6WWtOMU1FeHdXRWhDWjBk
      Vkt6VndTRFF3ZGswdmMzVlhjblJZYjNJckwydHRja3BFWkZWMU5IWkpaMmt4VW1aQmNsTmxabk5H
      TTFCdlIxWnFjUW8yWkhVM1RuQmhOazlQT1cxWFJGWXZPRnBqYW14SVdrVkpUVU5OYlRKamRqQnVk
      RmhuWTJwSmJrTmhlbmtyTVhkNmFHaExUMFJNV1RKTE9WZ3dPVUkzQ2xKR01HMWpjR0ZpUVZsclJH
      ZHpWVVYyU0RCM2RXUTRkRkpuZW5sWVFXWTJZMmw0TTBabVoyOTVTa2d2ZUdFd01uWkZRbFZGUWxG
      dFUzVlhjMEk0ZHpNS2FXaFVVVVpyVEN0NE1uQnliSGxXUWtWd2FqTnlRMmhZY3poMVVsbEJMeTlU
      UjNka1owSndkMlpVWm01ckswUk5VVGRuUzJKbFYyVnhabFFyVlRNNUt3cGFaMGhoTW1WbFkzZFJh
      MmhsVEUwNFkzVkhha012ZFVFNVpWSnhablJRY1RCcU0yVllNWGxGYjNOR1pHTldOekJuYzFJd1dH
      TjBSazlDWTJWSVVXNXRDak4xVjBSVVRqRmllV0kyWVZaYVFqZzVUM1JWTjFCU1preHhSMVkyYjBj
      MU5WQkZVRkV6VVRCSVRXMVRlRWt3WlRjNWNIUXhXVmxHUkhad2FtNXBjSGtLTjJ0aUwzbERObU5s
      ZFZGTWNraHpLMjVSYkVKQ05VSk5OV0ZQVFN0emMwSkpSM1JJZGtWUFRqWTFOMGw2WlRKaGNqRnBj
      bTh5Y1V4TVFXRkJVMHBrVEFwbU9ERmlPSGxFY1hCeVpFOVZaV1ZMYzBncldYazFhekZGVVUwdlRE
      QXpjeTkzVm1wa1NqQktiV2xFVlhwd1pXa3dkVXRCYmxGUWRFdElRbFJsYmtsSENtRnBMMU5KYlRO
      RWMxcHlNRTV4TnpsYVVWVjBiVEpYY0c5bGVrTkZla3ByVlN0eU4weFhjRWg1Y21SdE5VMVpOVzV4
      TW1sb2JsaEVNRFZHTkhsbGIwVUtObmh4VlRGYWJDdERXSEZOYTNOblJUWklZV0l2VDFscVpHRmFX
      VUZwUm5aUlJFOXhPVkU1V1ZVNE5rSnRXSGRwUzNoalltNVRWREJ6VnpZeFVUaFpWd3BWVUVSblpT
      dFJMMHhSUFQwS1BVVnpWVWNLTFMwdExTMUZUa1FnVUVkUUlGTkpSMDVCVkZWU1JTMHRMUzB0Q2c9
      PQ==
  tasks:
    - ping:
`)

		// should be the input but with the signature stripped
		// the "become" boolean should also be set to strictly true/false
		want := []byte(`- name: Insights Disable
  hosts: localhost
  become: true
  vars: {}
  tasks:
  - name: Disable the insights-client
    command: insights-client --disable-schedule
- name: ping
  hosts: localhost
  vars: {}
  tasks:
  - ping: null
`)

		got, err := stripSignature(playbook)
		if err != nil {
			t.Error(err)
		}
		if !cmp.Equal(got, want) {
			t.Errorf("\ngot:\n%v\nwant:\n%v", string(got), string(want))
		}
	})

	t.Run("stripSignature returns an error when given invalid YAML", func(t *testing.T) {

		got, err := stripSignature([]byte(`401 Unauthorized`))
		if err == nil {
			t.Error("there should have been a YAML parsing error")
		}
		if got != nil {
			t.Error("first returned value should have been nil")
		}
	})
}
