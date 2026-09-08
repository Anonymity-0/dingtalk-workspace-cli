// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package app

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
)

func TestCrossPlatformCoverageChatRecentConversationsSilentAlias(t *testing.T) {
	const primary = "chat +recent-conversations"
	const canonical = "chat.shortcut_active_conversations"
	const alias = "chat +active-conversations"

	root := NewRootCommand()
	leaf, remaining, err := root.Find([]string{"chat", "+recent-conversations"})
	if err != nil || len(remaining) != 0 || leaf == nil || !leaf.Runnable() || leaf.Name() != "+recent-conversations" {
		t.Fatalf("primary is not an exact runnable leaf: leaf=%v remaining=%v err=%v", leaf, remaining, err)
	}
	oldLeaf, remaining, err := root.Find([]string{"chat", "+active-conversations"})
	if err != nil || len(remaining) != 0 || oldLeaf != leaf {
		t.Fatalf("compatibility alias must resolve to the same Cobra leaf: leaf=%v remaining=%v err=%v", oldLeaf, remaining, err)
	}
	if !reflect.DeepEqual(leaf.Aliases, []string{"+active-conversations"}) || leaf.Hidden || leaf.Deprecated != "" {
		t.Fatalf("primary must remain visible with a non-deprecated compatibility alias: aliases=%v hidden=%v deprecated=%q", leaf.Aliases, leaf.Hidden, leaf.Deprecated)
	}
	for _, path := range []string{primary, alias} {
		meta, ok := cli.ResolveMeta(path)
		if !ok || meta.Identity.Canonical != canonical || !reflect.DeepEqual(meta.Identity.Aliases, []string{alias}) {
			t.Fatalf("metadata lost stable identity or compatibility alias: %#v, ok=%v", meta, ok)
		}
	}

	for _, compact := range []bool{false, true} {
		var primaryPayload map[string]any
		for _, tc := range []struct {
			name     string
			query    []string
			wantPath string
			isAlias  bool
		}{
			{"primary", []string{"--cli-path", primary}, primary, false},
			{"alias", []string{"--cli-path", alias}, alias, true},
			{"stable_identity", []string{canonical}, primary, false},
		} {
			root := NewRootCommand()
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			args := append([]string{"schema"}, tc.query...)
			args = append(args, "--format", "json")
			if compact {
				args = append(args, "--compact")
			}
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatalf("schema %s compact=%v: %v; %s", tc.name, compact, err, stderr.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			isAlias, _ := payload["is_alias"].(bool)
			if payload["canonical_path"] != canonical || payload["cli_path"] != tc.wantPath || (!compact && isAlias != tc.isAlias) {
				t.Fatalf("schema %s compact=%v: canonical=%v cli_path=%v is_alias=%v", tc.name, compact, payload["canonical_path"], payload["cli_path"], payload["is_alias"])
			}
			if !compact && (payload["primary_cli_path"] != primary || !reflect.DeepEqual(payload["aliases"], []any{alias})) {
				t.Fatalf("schema %s lost primary/alias mapping: primary=%v aliases=%v", tc.name, payload["primary_cli_path"], payload["aliases"])
			}
			examples, ok := payload["examples"].([]any)
			if !ok || len(examples) == 0 {
				t.Fatalf("schema %s must recommend primary examples", tc.name)
			}
			for _, example := range examples {
				if !strings.HasPrefix(example.(string), "dws "+primary) {
					t.Fatalf("schema %s recommends a non-primary example: %v", tc.name, example)
				}
			}
			description, _ := payload["description"].(string)
			if strings.Contains(description, "弃用") || strings.Contains(strings.ToLower(description), "deprecated") {
				t.Fatalf("schema %s compact=%v must not emit deprecation guidance: description=%q", tc.name, compact, description)
			}
			delete(payload, "cli_path")
			delete(payload, "is_alias")
			if tc.name == "primary" {
				primaryPayload = payload
			} else if !reflect.DeepEqual(payload, primaryPayload) {
				t.Fatalf("schema %s changed the contract beyond alias view fields (compact=%v)", tc.name, compact)
			}
		}
	}

	for _, name := range []string{"", "+recent-conversations", "+active-conversations"} {
		root := NewRootCommand()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		args := []string{"chat"}
		if name != "" {
			args = append(args, name)
		}
		root.SetArgs(append(args, "--help"))
		if err := root.Execute(); err != nil {
			t.Fatalf("help %q: %v; %s", name, err, stderr.String())
		}
		help := stdout.String()
		if strings.Contains(help, "弃用") || strings.Contains(strings.ToLower(help), "deprecated") || stderr.Len() != 0 {
			t.Fatalf("help %q emitted deprecation guidance: stdout=%s stderr=%s", name, help, stderr.String())
		}
		if !strings.Contains(help, "+recent-conversations") {
			t.Fatalf("help %q does not recommend the primary", name)
		}
		if name != "" && (strings.Contains(help, "--checkpoint") || strings.Contains(help, "--resume")) {
			t.Fatalf("help %q advertises removed persistence flags", name)
		}
		if name == "" {
			if strings.Contains(help, "+active-conversations") {
				t.Fatal("product help advertises the compatibility alias")
			}
		} else if !strings.Contains(help, "dws "+primary+" [flags]") {
			t.Fatalf("exact help %q must show primary usage", name)
		}
	}
}

func TestCrossPlatformCoverageChatRecentConversationsRejectsRemovedPersistenceFlags(t *testing.T) {
	for _, name := range []string{"+recent-conversations", "+active-conversations"} {
		for _, flag := range []string{"checkpoint", "resume"} {
			t.Run(name+"/"+flag, func(t *testing.T) {
				root := NewRootCommand()
				leaf, _, err := root.Find([]string{"chat", name})
				if err != nil {
					t.Fatal(err)
				}
				// Parse only: removed flags must fail before any auth or RPC work.
				if err := leaf.ParseFlags([]string{"--" + flag, "progress.json"}); err == nil || !strings.Contains(err.Error(), "unknown flag") {
					t.Fatalf("removed --%s is still accepted: %v", flag, err)
				}
			})
		}
	}
}
