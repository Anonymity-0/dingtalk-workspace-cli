// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package responsecheck

import "testing"

func TestCrossPlatformCoverageRequireObjectCollection(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantLen int
		wantErr bool
	}{
		{name: "non-empty", data: map[string]any{"success": true, "result": []any{map[string]any{"id": "one"}}}, wantLen: 1},
		{name: "content envelope", data: map[string]any{"content": map[string]any{"success": true, "result": []any{map[string]any{"id": "one"}}}}, wantLen: 1},
		{name: "explicit empty", data: map[string]any{"success": true, "result": []any{}}, wantLen: 0},
		{name: "missing collection", data: map[string]any{"success": true, "result": map[string]any{}}, wantErr: true},
		{name: "wrong collection type", data: map[string]any{"success": true, "result": "bad"}, wantErr: true},
		{name: "malformed item", data: map[string]any{"success": true, "result": []any{"bad"}}, wantErr: true},
		{name: "empty item", data: map[string]any{"success": true, "result": []any{map[string]any{}}}, wantErr: true},
		{name: "missing success", data: map[string]any{"result": []any{}}, wantErr: true},
		{name: "remote failure", data: map[string]any{"success": false, "result": []any{}}, wantErr: true},
		{name: "empty response", data: map[string]any{}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RequireObjectCollection(tc.data, "test/read", "result")
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tc.wantLen)
			}
		})
	}
}

func TestCrossPlatformCoverageRequireObjectCollectionNested(t *testing.T) {
	data := map[string]any{
		"success": true,
		"result": map[string]any{
			"items": []any{map[string]any{"id": "one"}},
		},
	}
	items, err := RequireObjectCollection(data, "test/nested", "result.items")
	if err != nil {
		t.Fatalf("RequireObjectCollection: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v", items)
	}
}

func TestCrossPlatformCoverageRequireResultRejectsNull(t *testing.T) {
	if _, err := RequireResult(map[string]any{"success": true, "result": nil}, "test/null"); err == nil {
		t.Fatal("expected null result to fail closed")
	}
	if _, err := RequireObjectResult(map[string]any{"success": true, "result": []any{}}, "test/object"); err == nil {
		t.Fatal("expected array result to fail object validation")
	}
}

func TestCrossPlatformCoverageRequireSingleObjectResult(t *testing.T) {
	for _, data := range []map[string]any{
		{"success": true, "result": map[string]any{"id": "one"}},
		{"success": true, "result": []any{map[string]any{"id": "one"}}},
	} {
		object, err := RequireSingleObjectResult(data, "test/detail")
		if err != nil || object["id"] != "one" {
			t.Fatalf("valid detail shape rejected: object=%v err=%v", object, err)
		}
	}
	for _, data := range []map[string]any{
		{"success": true, "result": map[string]any{}},
		{"success": true, "result": []any{}},
		{"success": true, "result": []any{map[string]any{"id": "one"}, map[string]any{"id": "two"}}},
		{"success": true, "result": []any{"bad"}},
	} {
		if object, err := RequireSingleObjectResult(data, "test/detail"); err == nil {
			t.Fatalf("ambiguous/malformed detail returned success: %v", object)
		}
	}
}
