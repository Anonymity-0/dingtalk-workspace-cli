// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package minutesdata

import "testing"

func TestCrossPlatformCoverageMinutesTodosTruthStates(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		wantState  ArtifactState
		wantSource string
		wantCount  int
	}{
		{
			name: "ready prefers structured todos",
			data: map[string]any{"success": true, "result": map[string]any{
				"actions":          []any{"legacy"},
				"dingtalkTodoList": []any{map[string]any{"minutesTodoId": "t1"}},
			}},
			wantState: ArtifactReady, wantSource: "dingtalkTodoList", wantCount: 1,
		},
		{
			name: "ready actions fallback",
			data: map[string]any{"success": true, "result": map[string]any{
				"actions": []any{"one", "two"},
			}},
			wantState: ArtifactReady, wantSource: "actions", wantCount: 2,
		},
		{
			name: "known empty",
			data: map[string]any{"success": true, "result": map[string]any{
				"actions": []any{},
			}},
			wantState: ArtifactKnownEmpty, wantSource: "actions", wantCount: 0,
		},
		{
			name: "unknown shape",
			data: map[string]any{"success": true, "result": map[string]any{
				"secret": "must-not-copy",
			}},
			wantState: ArtifactUnsupportedShape,
		},
		{
			name: "wrong collection types",
			data: map[string]any{"success": true, "result": map[string]any{
				"actions": "bad", "dingtalkTodoList": map[string]any{},
			}},
			wantState: ArtifactUnsupportedShape,
		},
		{
			name:      "backend failure",
			data:      map[string]any{"success": false, "errorMsg": "denied"},
			wantState: ArtifactFailed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fact := InspectTodos("u1", test.data)
			if fact.State != test.wantState || fact.SourceField != test.wantSource || len(fact.Items) != test.wantCount {
				t.Fatalf("fact=%#v", fact)
			}
			payload := fact.Payload()
			if payload["state"] != string(test.wantState) || payload["taskUuid"] != "u1" || payload["itemCount"] != test.wantCount {
				t.Fatalf("payload=%#v", payload)
			}
			if fact.Successful() != (test.wantState == ArtifactReady || test.wantState == ArtifactKnownEmpty) {
				t.Fatalf("successful=%v state=%s", fact.Successful(), fact.State)
			}
			if test.wantState == ArtifactUnsupportedShape && payload["secret"] != nil {
				t.Fatalf("unsupported payload leaked raw value: %#v", payload)
			}
		})
	}
}

func TestCrossPlatformCoverageMinutesTodosTruthDiagnostics(t *testing.T) {
	fact := InspectTodos("u1", map[string]any{
		"result": map[string]any{"actions": "bad", "dingtalkTodoList": map[string]any{}},
	})
	if fact.Err() == nil || fact.Successful() {
		t.Fatalf("unsupported fact accepted: %#v", fact)
	}
	ledger := fact.Ledger()
	fields := ledger["observedFields"].([]string)
	types := ledger["observedTypes"].(map[string]string)
	if len(fields) != 2 || fields[0] != "actions" || fields[1] != "dingtalkTodoList" || types["actions"] != "string" || types["dingtalkTodoList"] != "object" {
		t.Fatalf("ledger=%#v", ledger)
	}
	if ledger["complete"] != false || ledger["retryable"] != false {
		t.Fatalf("failure flags=%#v", ledger)
	}

	failed := FailedTodos("u2", nil)
	if failed.State != ArtifactFailed || failed.Err() == nil || failed.Payload()["taskUuid"] != "u2" {
		t.Fatalf("failed=%#v", failed)
	}
}
