package main

import (
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

func TestValidateDirectRequestRequiresConcreteSubject(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		wantErr bool
	}{
		{name: "concrete", subject: "job.echo"},
		{name: "empty token", subject: "job..echo", wantErr: true},
		{name: "leading empty token", subject: ".job", wantErr: true},
		{name: "trailing empty token", subject: "job.", wantErr: true},
		{name: "embedded star", subject: "job.ech*o", wantErr: true},
		{name: "embedded greater-than", subject: "job.>x", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &agentv1.JobRequest{JobId: "job-1", Topic: test.subject}
			err := validateDirectRequest(req)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateDirectRequest(%q) error = %v, wantErr %v", test.subject, err, test.wantErr)
			}
		})
	}
}
