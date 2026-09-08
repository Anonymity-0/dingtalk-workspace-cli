// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package minutes

import (
	"reflect"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

const speakerReadyFixture = `{"success":true,"result":{"status":"completed","innerStatus":"Finished","success":true,"content":"Synthetic speaker summary","errorMsg":"","taskId":"job"}}`

func TestCrossPlatformCoverageMinutesSpeakerTerminalStates(t *testing.T) {
	for _, tc := range []struct {
		name, response, state string
		wantErr               bool
	}{
		{"ready", speakerReadyFixture, "ready", false},
		{"pending", `{"success":true,"result":{"status":"processing","taskId":"job"}}`, "pending", true},
		{"unknown", `{"success":true,"result":{"anything":"nonempty"}}`, "unsupported_shape", true},
		{"failed", `{"success":true,"result":{"success":false,"errorMsg":"failed"}}`, "failed", true},
		{"mismatch", `{"success":true,"result":{"status":"processing","taskId":"other"}}`, "unsupported_shape", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &minutesE2ECaller{responses: map[string][]string{"minutes/get_speaker_summary": {tc.response}}}
			p, _, err := runMinutesAlignmentCLI(t, c, "minutes", "+speaker-insights", "--id", "u1", "--resume", "--task-id", "job", "--timeout", "1", "--interval", "1", "--yes")
			if (err != nil) != tc.wantErr || p["state"] != tc.state || p["complete"] != (tc.state == "ready") {
				t.Fatalf("payload=%#v err=%v", p, err)
			}
			if c.counts["minutes/create_speaker_summary"] != 0 || c.counts["minutes/get_speaker_summary"] != 1 {
				t.Fatalf("calls=%v", c.counts)
			}
			if tc.wantErr && (p["taskId"] != "job" || p["recovery"] == nil) {
				t.Fatalf("lost recovery: %#v", p)
			}
		})
	}
	apiErr := apperrors.NewAPI("downstream query empty", apperrors.WithReason("business_error"), apperrors.WithServerDiag(apperrors.ServerDiagnostics{ServerErrorCode: "000"}))
	if !speakerSummaryPending(apiErr) {
		t.Fatal("observed structured error not recognized")
	}
	otherErr := apperrors.NewAPI("permission denied processing", apperrors.WithReason("business_error"), apperrors.WithServerDiag(apperrors.ServerDiagnostics{ServerErrorCode: "000"}))
	if speakerSummaryPending(otherErr) {
		t.Fatal("unrelated error retried")
	}
	c := &minutesE2ECaller{}
	if _, _, err := runMinutesAlignmentCLI(t, c, "minutes", "+speaker-insights", "--id", "u1", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if len(c.counts) != 0 {
		t.Fatal("dry run made remote calls")
	}
}

func TestCrossPlatformCoverageMinutesSpeakerUnavailableRecovery(t *testing.T) {
	unavailable := apperrors.NewAPI("downstream query empty", apperrors.WithReason("business_error"), apperrors.WithServerDiag(apperrors.ServerDiagnostics{ServerErrorCode: "000"}))
	c := &minutesE2ECaller{
		responses:  map[string][]string{"minutes/get_speaker_summary": {speakerReadyFixture}},
		failAt:     map[string]int{"minutes/get_speaker_summary": 1},
		failErrors: map[string]error{"minutes/get_speaker_summary": unavailable},
	}
	p, _, err := runMinutesAlignmentCLI(t, c, "minutes", "+speaker-insights", "--id", "u1", "--resume", "--task-id", "job", "--timeout", "3", "--interval", "1", "--yes")
	if err != nil || p["state"] != "ready" || p["attempts"] != float64(2) || c.counts["minutes/create_speaker_summary"] != 0 {
		t.Fatalf("payload=%#v calls=%v err=%v", p, c.counts, err)
	}
	c = &minutesE2ECaller{responses: map[string][]string{"minutes/get_speaker_summary": {`{"success":true,"result":{"status":"processing","taskId":"job"}}`}}}
	p, _, err = runMinutesAlignmentCLI(t, c, "minutes", "+speaker-insights", "--id", "u1", "--resume", "--task-id", "job", "--timeout", "1", "--interval", "1", "--profile", "corp:user", "--yes")
	if err == nil || p["retryable"] != true {
		t.Fatalf("timeout = %#v, %v", p, err)
	}
	recovery := p["recovery"].(map[string]any)
	want := []any{"dws", "minutes", "+speaker-insights", "--id", "u1", "--resume", "--task-id", "job", "--timeout", "1", "--interval", "1", "--profile", "corp:user"}
	if !reflect.DeepEqual(recovery["nextCommand"], want) {
		t.Fatalf("recovery=%#v", recovery)
	}
	c = &minutesE2ECaller{}
	if _, _, err = runMinutesAlignmentCLI(t, c, "minutes", "+speaker-insights", "--id", "u1"); err == nil || len(c.counts) != 0 {
		t.Fatal("confirmation gate crossed")
	}
}
