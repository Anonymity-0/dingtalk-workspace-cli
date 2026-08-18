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

func TestCrossPlatformCoverageMailSemanticCatalogExactlyCoversRegisteredSurface(t *testing.T) {
	raw, err := os.ReadFile("../semantic_catalog_mail.json")
	if err != nil {
		t.Fatal(err)
	}
	var source chatSemanticCatalogFixture
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	if source.Service != "mail" {
		t.Fatalf("service = %q", source.Service)
	}
	registered := map[string]shortcut.Shortcut{}
	for _, item := range shortcut.All() {
		if item.Service == "mail" {
			registered[item.Command] = item
		}
	}
	if len(registered) != 18 || len(source.Shortcuts) != 18 {
		t.Fatalf("registered/catalog = %d/%d, want 18/18", len(registered), len(source.Shortcuts))
	}
	var missing, stale []string
	public := 0
	unavailable := 0
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
		}
		if availability == shortcut.AvailabilityUnavailable {
			unavailable++
			if !item.Hidden || item.Availability != shortcut.AvailabilityUnavailable {
				t.Errorf("%s: unavailable shortcut remains visible", command)
			}
			if item.Contract.Interface == nil || item.Contract.Interface.Availability != "unavailable" || strings.TrimSpace(item.Contract.Interface.Reason) == "" {
				t.Errorf("%s: unavailable runtime interface is not explicit", command)
			}
			if item.Contract.Result != nil || item.Contract.Pagination != nil || item.OutputRollout != output.RolloutLegacyOnly {
				t.Errorf("%s: unavailable runtime still publishes result/pagination/unified rollout", command)
			}
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
	if public != 8 || unavailable != 10 {
		t.Fatalf("mail public/unavailable shortcuts = %d/%d, want 8/10", public, unavailable)
	}
}
