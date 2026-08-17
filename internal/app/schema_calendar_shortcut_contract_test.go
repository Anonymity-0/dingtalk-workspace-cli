// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package app

import "testing"

func TestCrossPlatformCoverageCalendarAgendaFinalSchemaPreservesCompositeProperties(t *testing.T) {
	snapshot := fullSchemaSnapshotForTest(t)
	tool := snapshot.Tools["calendar.shortcut_agenda"]
	if tool == nil {
		t.Fatal("calendar.shortcut_agenda is missing from final Schema")
	}
	parameters := schemaContractMap(tool["parameters"])
	for flag, want := range map[string]string{
		"start": "start",
		"end":   "end",
	} {
		parameter := parameters[flag]
		if parameter == nil {
			t.Fatalf("calendar.shortcut_agenda --%s is missing from final Schema", flag)
		}
		if got := schemaContractString(parameter["property"]); got != want {
			t.Errorf("calendar.shortcut_agenda --%s property=%q, want %q", flag, got, want)
		}
	}
	if got := schemaContractString(tool["interface_mode"]); got != "composite" {
		t.Fatalf("calendar.shortcut_agenda interface_mode=%q, want composite", got)
	}
}
