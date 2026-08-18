// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package builtin_test

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func TestCrossPlatformCoverageAttendanceSemanticCatalogExactlyCoversRegisteredSurface(t *testing.T) {
	raw, err := os.ReadFile("../semantic_catalog_attendance.json")
	if err != nil {
		t.Fatal(err)
	}
	var source chatSemanticCatalogFixture
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	if source.Service != "attendance" {
		t.Fatalf("service = %q", source.Service)
	}
	registered := map[string]shortcut.Shortcut{}
	for _, item := range shortcut.All() {
		if item.Service != "attendance" {
			continue
		}
		if _, exists := registered[item.Command]; exists {
			t.Fatalf("duplicate %s", item.Command)
		}
		registered[item.Command] = item
	}
	if len(registered) != 35 || len(source.Shortcuts) != 35 {
		t.Fatalf("registered/catalog = %d/%d, want 35/35", len(registered), len(source.Shortcuts))
	}
	public := 0
	var missing, stale []string
	for command, item := range registered {
		record, ok := source.Shortcuts[command]
		if !ok {
			missing = append(missing, command)
			continue
		}
		availability := record.Availability
		if availability == "" {
			availability = source.Availability
		}
		if !record.Reviewed || !item.SemanticReviewed || strings.TrimSpace(item.SemanticDelta) == "" || item.SemanticDelta != record.SemanticDelta {
			t.Errorf("%s: semantic review facts drifted", command)
		}
		if item.Risk != record.Risk {
			t.Errorf("%s: risk=%q want=%q", command, item.Risk, record.Risk)
		}
		if record.Public {
			public++
			if availability != shortcut.AvailabilityAvailable || item.Hidden || item.Availability != shortcut.AvailabilityAvailable {
				t.Errorf("%s: public available shortcut is not executable/visible", command)
			}
			if item.Contract.Empty() || item.Contract.Result == nil || strings.TrimSpace(item.Safety.Effect) == "" || item.OutputRollout != output.RolloutUnifiedActive {
				t.Errorf("%s: public shortcut lacks contract/safety/result/unified output", command)
			}
		} else if availability == shortcut.AvailabilityAvailable || !item.Hidden {
			t.Errorf("%s: unavailable shortcut must stay hidden", command)
		}
	}
	for command := range source.Shortcuts {
		if _, ok := registered[command]; !ok {
			stale = append(stale, command)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 || len(stale) > 0 {
		t.Fatalf("catalog mismatch: missing=%v stale=%v", missing, stale)
	}
	if public != 9 {
		t.Fatalf("public attendance shortcuts = %d, want 9", public)
	}
}
