// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package minutes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCrossPlatformCoverageMinutesUploadPlanOptions(t *testing.T) {
	file := filepath.Join(t.TempDir(), "audio.wav")
	if err := os.WriteFile(file, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"+upload", "+upload-and-notify", "+upload-and-analyze"} {
		caller := &minutesE2ECaller{}
		p, _, err := runMinutesAlignmentCLI(t, caller, "minutes", command, "--file", file, "--title", " meeting ", "--input-language", "zh", "--template-id", "template-A", "--complete-timeout", "120", "--poll-interval", "4", "--dry-run")
		if err != nil || len(caller.counts) != 0 || p["executed"] != false {
			t.Fatalf("plan=%#v err=%v", p, err)
		}
		if command == "+upload-and-analyze" {
			p = p["upload"].(map[string]any)
		}
		options := p["options"].(map[string]any)
		if options["inputLanguage"] != "zh" || options["templateId"] != "template-A" || p["title"] != "meeting" || p["completeTimeoutSeconds"] != float64(120) || p["pollIntervalSeconds"] != float64(4) {
			t.Fatalf("lost options=%#v", p)
		}
		if command == "+upload-and-notify" && options["enableMessageCard"] != true {
			t.Fatal("lost notification setting")
		}
	}
	c := &minutesE2ECaller{}
	p, _, err := runMinutesAlignmentCLI(t, c, "minutes", "+upload", "--file", file, "--dry-run")
	if err != nil || len(c.counts) != 0 || len(p["options"].(map[string]any)) != 0 || p["completeTimeoutSeconds"] != float64(90) || p["pollIntervalSeconds"] != float64(2) {
		t.Fatalf("defaults=%#v err=%v", p, err)
	}
	for _, args := range [][]string{
		{"minutes", "+upload", "--file", file, "--complete-timeout", "0", "--dry-run"},
		{"minutes", "+upload", "--file", filepath.Join(t.TempDir(), "missing.wav"), "--dry-run"},
	} {
		c := &minutesE2ECaller{}
		if _, _, err := runMinutesAlignmentCLI(t, c, args...); err == nil || len(c.counts) != 0 {
			t.Fatal("invalid plan did not fail locally")
		}
	}
}
