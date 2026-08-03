// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateManifestAcceptsExactReviewedRoute(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "skills/multi/dingtalk-chat/SKILL.md", "| 发消息 | `dws chat +send` | <!-- dws-intent: chat.send -->\n")
	writeTestFile(t, root, "internal/cli/schema_hints/selection/chat.json", `{
  "tools": {
    "chat.shortcut_send": {"use_when":["普通发送"],"avoid_when":["底层字段另选"]},
    "chat.atomic_send": {"use_when":["底层字段"],"avoid_when":["普通发送使用 +send"]}
  }
}`)
	writeTypedContractReference(t, root, "skills/multi/dingtalk-chat/references/contracts.md")
	writeEventHandoffReference(t, root, "skills/multi/dingtalk-event/references/event-im-output.md")
	manifest := routeManifest{
		Version:           2,
		MarkerRoots:       []string{"skills/multi/dingtalk-chat"},
		RetiredScripts:    []string{"skills/multi/dingtalk-chat/scripts/retired.py"},
		ContractReference: "skills/multi/dingtalk-chat/references/contracts.md",
		HandoffReference:  "skills/multi/dingtalk-event/references/event-im-output.md",
		Intents: []intentRoute{{
			ID:                    "chat.send",
			PreferredTool:         "chat.shortcut_send",
			ForbiddenDefaultTools: []string{"chat.atomic_send"},
			SelectionFile:         "internal/cli/schema_hints/selection/chat.json",
			References:            []string{"skills/multi/dingtalk-chat/SKILL.md"},
		}},
	}
	tools := map[string]toolFact{
		"chat.shortcut_send": {Canonical: "chat.shortcut_send", PrimaryPath: "chat +send", Confirmation: "user_required", HasMeta: true},
		"chat.atomic_send":   {Canonical: "chat.atomic_send", PrimaryPath: "chat message send", Confirmation: "not_required", HasMeta: true},
	}
	if failures := validateManifest(root, manifest, tools); len(failures) != 0 {
		t.Fatalf("failures = %v", failures)
	}
}

func TestValidateManifestRejectsWrongMarkerAndSafetyDowngrade(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "skills/multi/dingtalk-chat/SKILL.md", "| 发消息 | `dws chat message send` | <!-- dws-intent: chat.send -->\n")
	writeTestFile(t, root, "internal/cli/schema_hints/selection/chat.json", `{
  "tools": {
    "chat.shortcut_send": {"use_when":["普通发送"],"avoid_when":["底层字段另选"]},
    "chat.atomic_send": {"use_when":["普通发送"],"avoid_when":["继续使用原子命令"]}
  }
}`)
	writeTypedContractReference(t, root, "skills/multi/dingtalk-chat/references/contracts.md")
	writeEventHandoffReference(t, root, "skills/multi/dingtalk-event/references/event-im-output.md")
	manifest := routeManifest{
		Version:           2,
		MarkerRoots:       []string{"skills/multi/dingtalk-chat"},
		RetiredScripts:    []string{"skills/multi/dingtalk-chat/scripts/retired.py"},
		ContractReference: "skills/multi/dingtalk-chat/references/contracts.md",
		HandoffReference:  "skills/multi/dingtalk-event/references/event-im-output.md",
		Intents: []intentRoute{{
			ID:                    "chat.send",
			PreferredTool:         "chat.shortcut_send",
			AllowedFallbacks:      []routeFallback{{Tool: "chat.atomic_send", ReasonCode: "raw_field"}},
			ForbiddenDefaultTools: []string{"chat.atomic_send"},
			SelectionFile:         "internal/cli/schema_hints/selection/chat.json",
			References:            []string{"skills/multi/dingtalk-chat/SKILL.md"},
		}},
	}
	tools := map[string]toolFact{
		"chat.shortcut_send": {Canonical: "chat.shortcut_send", PrimaryPath: "chat +send", Confirmation: "user_required", HasMeta: true},
		"chat.atomic_send":   {Canonical: "chat.atomic_send", PrimaryPath: "chat message send", Confirmation: "not_required", HasMeta: true},
	}
	failures := strings.Join(validateManifest(root, manifest, tools), "\n")
	for _, want := range []string{"confirmation", "does not route ordinary use", "must contain preferred path", "uses forbidden default"} {
		if !strings.Contains(failures, want) {
			t.Fatalf("failures = %s, want %q", failures, want)
		}
	}
}

func TestValidateSelectionSourceRefsRejectsStaleRegistryAndMissingAnchor(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "internal/cli/schema_command_registry/products/chat.json", `{"tools":[{"canonical_path":"chat.exists"}]}`)
	selection := selectionFile{Tools: map[string]selectionEntry{
		"chat.sample": {SourceRefs: []string{
			"internal/cli/schema_command_registry.json#chat.sample",
			"internal/cli/schema_command_registry/products/chat.json#chat.missing",
		}},
	}}
	failures := strings.Join(validateSelectionSourceRefs(root, "selection/chat.json", selection, map[string]map[string]bool{}), "\n")
	if !strings.Contains(failures, "stale source_ref") || !strings.Contains(failures, "anchor") {
		t.Fatalf("failures = %s", failures)
	}
}

func TestLoadManifestRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRetiredScriptsRejectsRepublishedAndDuplicatePaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "skills/multi/dingtalk-chat/scripts/export.py", "republished\n")
	failures := strings.Join(validateRetiredScripts(root, []string{
		"skills/multi/dingtalk-chat/scripts/export.py",
		"skills/multi/dingtalk-chat/scripts/export.py",
		"../unsafe.py",
	}), "\n")
	for _, want := range []string{"was republished", "duplicate retired", "invalid retired"} {
		if !strings.Contains(failures, want) {
			t.Fatalf("failures = %s, want %q", failures, want)
		}
	}
}

func TestValidateTypedContractReferenceRejectsDrift(t *testing.T) {
	root := t.TempDir()
	writeTypedContractReference(t, root, "contracts.md")
	path := filepath.Join(root, "contracts.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "`im.message-list.v1`", "`drifted`", 1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	failures := strings.Join(validateTypedContractReference(root, "contracts.md"), "\n")
	if !strings.Contains(failures, "MESSAGE_RESULT contract differs") {
		t.Fatalf("failures = %s", failures)
	}
}

func TestValidateEventHandoffReferenceRejectsNaturalTargetDrift(t *testing.T) {
	root := t.TempDir()
	writeEventHandoffReference(t, root, "handoff.md")
	path := filepath.Join(root, "handoff.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "--group <conversation_id>", "--chat-query <display_name>", 1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	failures := strings.Join(validateEventHandoffReference(root, "handoff.md"), "\n")
	if !strings.Contains(failures, "exact stable-ID mapping") {
		t.Fatalf("failures = %s", failures)
	}
}

func writeTypedContractReference(t *testing.T, root, relative string) {
	t.Helper()
	var content strings.Builder
	for _, block := range []struct {
		name string
		body string
	}{
		{name: "MESSAGE_RESULT", body: renderMessageResultContract()},
		{name: "IDENTITY_CAPABILITY", body: renderIdentityCapabilityContract()},
		{name: "CARD_WORKFLOW", body: renderCardWorkflowContract()},
		{name: "CAPABILITY_BOUNDARY", body: renderCapabilityBoundaryContract()},
	} {
		content.WriteString("<!-- DWS_" + block.name + "_CONTRACT_START -->\n")
		content.WriteString(block.body)
		content.WriteString("\n<!-- DWS_" + block.name + "_CONTRACT_END -->\n")
	}
	writeTestFile(t, root, relative, content.String())
}

func writeEventHandoffReference(t *testing.T, root, relative string) {
	t.Helper()
	writeTestFile(t, root, relative, `<!-- DWS_EVENT_CHAT_HANDOFF_START -->
| event field | exact chat target |
|---|---|
| `+"`conversation_id`"+` | `+"`dws chat +messages-send --as user --group <conversation_id>`"+` |
| `+"`sender_open_dingtalk_id`"+` | `+"`dws chat +messages-send --as user --open-dingtalk-id <sender_open_dingtalk_id>`"+` |
<!-- DWS_EVENT_CHAT_HANDOFF_END -->
`)
}

func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
