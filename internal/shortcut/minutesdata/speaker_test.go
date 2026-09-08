// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package minutesdata

import "testing"

func TestCrossPlatformCoverageSpeakerSummaryEvidence(t *testing.T) {
	if p := ParseSpeakerSummary(map[string]any{"result": map[string]any{"status": "processing", "taskId": "job"}}); p.State != SpeakerPending {
		t.Fatalf("pending = %#v", p)
	}
	ready := map[string]any{"status": "completed", "innerStatus": "Finished", "success": true, "content": "summary", "errorMsg": "", "taskId": "job"}
	if _, err := SpeakerSummaryResult(map[string]any{"result": ready}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key   string
		value any
	}{
		{"status", "processing"}, {"status", "unknown"}, {"innerStatus", "Running"}, {"content", " "}, {"content", 12}, {"errorMsg", "failed"}, {"success", false}, {"success", "true"}, {"taskId", ""},
	} {
		r := map[string]any{}
		for k, v := range ready {
			r[k] = v
		}
		r[tc.key] = tc.value
		if p := ParseSpeakerSummary(map[string]any{"result": r}); p.State == SpeakerReady {
			t.Fatalf("accepted %s=%v", tc.key, tc.value)
		}
	}
	for key := range ready {
		r := map[string]any{}
		for k, v := range ready {
			if k != key {
				r[k] = v
			}
		}
		if p := ParseSpeakerSummary(map[string]any{"result": r}); p.State == SpeakerReady {
			t.Fatalf("accepted missing %s", key)
		}
	}
	for _, data := range []map[string]any{{}, {"result": []any{}}, {"result": map[string]any{"summary": "text"}}, {"success": "true", "result": ready}, {"success": false, "result": ready}} {
		if p := ParseSpeakerSummary(data); p.State != SpeakerUnsupported {
			t.Fatalf("accepted %#v as %s", data, p.State)
		}
	}
}
