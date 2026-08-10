// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package devapp

import "testing"

func TestDevAppListProjectionPreservesPaginationEvidence(t *testing.T) {
	items := []map[string]any{{"id": "one"}}
	for _, tc := range []struct {
		name string
		data map[string]any
	}{
		{name: "top level", data: map[string]any{"hasMore": true, "nextCursor": "next"}},
		{name: "nested result", data: map[string]any{"result": map[string]any{"hasMore": true, "nextCursor": "next"}}},
		{name: "exhausted", data: map[string]any{"hasMore": false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := devAppListProjection(tc.data, "items", items)
			if got["hasMore"] != devAppPaginationCandidates(tc.data)[len(devAppPaginationCandidates(tc.data))-1]["hasMore"] && tc.name == "nested result" {
				t.Fatalf("hasMore not preserved: %#v", got)
			}
			if tc.name != "exhausted" && got["nextCursor"] != "next" {
				t.Fatalf("nextCursor not preserved: %#v", got)
			}
			if got["count"] != 1 {
				t.Fatalf("projection count=%#v", got["count"])
			}
		})
	}
}
