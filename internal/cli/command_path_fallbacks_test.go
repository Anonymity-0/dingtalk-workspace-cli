// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package cli

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func commandFallbackJSON(entries string) []byte {
	return []byte(`{"$schema":"./command_path_fallbacks.schema.json","version":1,"entries":` + entries + `}`)
}

func validCommandFallbackEntry(from, to string) string {
	return `{"from":"` + from + `","mode":"rewrite","to":"` + to + `","reviewed":true,"review_reason":"reviewed fixture"}`
}

func TestDecodeCommandPathFallbacksStrictContract(t *testing.T) {
	valid := commandFallbackJSON(`[` + validCommandFallbackEntry(`chat +bad`, `chat +good`) + `]`)
	entries, err := decodeCommandPathFallbacks(valid)
	if err != nil || len(entries) != 1 || entries[0].From != "chat +bad" || entries[0].To != "chat +good" {
		t.Fatalf("decode valid = %#v, %v", entries, err)
	}
	entries[0].Candidates = append(entries[0].Candidates, "mutated")
	cloned := cloneCommandPathFallbacks(entries)
	cloned[0].Candidates[0] = "clone"
	if entries[0].Candidates[0] != "mutated" {
		t.Fatal("cloneCommandPathFallbacks aliases candidate storage")
	}

	tests := map[string]struct {
		data []byte
		want string
	}{
		"unknown field": {
			data: commandFallbackJSON(`[{"from":"chat +bad","mode":"rewrite","to":"chat +good","reviewed":true,"review_reason":"x","extra":true}]`),
			want: "unknown field",
		},
		"wrong schema": {
			data: []byte(`{"$schema":"wrong","version":1,"entries":[` + validCommandFallbackEntry(`chat +bad`, `chat +good`) + `]}`),
			want: "$schema",
		},
		"wrong version": {
			data: []byte(`{"$schema":"./command_path_fallbacks.schema.json","version":2,"entries":[` + validCommandFallbackEntry(`chat +bad`, `chat +good`) + `]}`),
			want: "unsupported",
		},
		"empty entries": {data: commandFallbackJSON(`[]`), want: "no entries"},
		"unnormalized path": {
			data: commandFallbackJSON(`[` + validCommandFallbackEntry(` chat  +bad `, `chat +good`) + `]`),
			want: "not a normalized",
		},
		"unreviewed": {
			data: commandFallbackJSON(`[{"from":"chat +bad","mode":"rewrite","to":"chat +good","reviewed":false,"review_reason":"x"}]`),
			want: "reviewed=true",
		},
		"empty reason": {
			data: commandFallbackJSON(`[{"from":"chat +bad","mode":"rewrite","to":"chat +good","reviewed":true,"review_reason":" "}]`),
			want: "reviewed=true",
		},
		"invalid mode": {
			data: commandFallbackJSON(`[{"from":"chat +bad","mode":"guess","reviewed":true,"review_reason":"x"}]`),
			want: "invalid mode",
		},
		"rewrite candidates": {
			data: commandFallbackJSON(`[{"from":"chat +bad","mode":"rewrite","to":"chat +good","candidates":["chat +one","chat +two"],"reviewed":true,"review_reason":"x"}]`),
			want: "must not declare candidates",
		},
		"ambiguous target": {
			data: commandFallbackJSON(`[{"from":"chat +bad","mode":"ambiguous","to":"chat +good","candidates":["chat +one","chat +two"],"reviewed":true,"review_reason":"x"}]`),
			want: "must not declare to",
		},
		"ambiguous too small": {
			data: commandFallbackJSON(`[{"from":"chat +bad","mode":"ambiguous","candidates":["chat +one"],"reviewed":true,"review_reason":"x"}]`),
			want: "at least two",
		},
		"duplicate candidate": {
			data: commandFallbackJSON(`[{"from":"chat +bad","mode":"ambiguous","candidates":["chat +one","chat +one"],"reviewed":true,"review_reason":"x"}]`),
			want: "repeats candidate",
		},
		"duplicate from": {
			data: commandFallbackJSON(`[` + validCommandFallbackEntry(`chat +bad`, `chat +good`) + `,` + validCommandFallbackEntry(`chat +bad`, `chat +other`) + `]`),
			want: "duplicate from",
		},
		"multiple documents": {
			data: append(valid, []byte(` {}`)...),
			want: "multiple JSON values",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeCommandPathFallbacks(test.data); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("decode error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestCommandPathFallbackSchemaAndEmbeddedSource(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(embeddedCommandPathFallbacksSchemaJSON, &schema); err != nil {
		t.Fatalf("decode embedded schema: %v", err)
	}
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" || schema["additionalProperties"] != false {
		t.Fatalf("schema header = %#v", schema)
	}
	entries, err := LoadCommandPathFallbacks()
	if err != nil || len(entries) != 20 {
		t.Fatalf("LoadCommandPathFallbacks() = %#v, %v", entries, err)
	}
	if got, ok := LookupCommandPathFallback("dws chat +search-group"); !ok || got.To != "chat +chat-search" {
		t.Fatalf("generated lookup = %#v, %v", got, ok)
	}
	if _, ok := LookupCommandPathFallback("chat +missing"); ok {
		t.Fatal("generated lookup accepted unknown path")
	}
}

func TestCommandPathFallbackAuditCoverage(t *testing.T) {
	rewrites := map[string]string{
		"chat +search-group":       "chat +chat-search",
		"chat +group-search":       "chat +chat-search",
		"chat +chat-group-search":  "chat +chat-search",
		"chat +chat-group-members": "chat +chat-members-list",
		"chat +members":            "chat +group-members",
		"chat +group-member-list":  "chat +group-members",
		"chat +list-group-bots":    "chat +chat-bots",
		"chat +list-robot":         "chat +chat-bots",
		"chat +list-robots":        "chat +chat-bots",
		"chat group search":        "chat search",
	}
	for from, to := range rewrites {
		entry, ok := LookupCommandPathFallback(from)
		if !ok || entry.Mode != CommandPathFallbackRewrite || entry.To != to {
			t.Errorf("rewrite %q = %#v, %v; want %q", from, entry, ok, to)
		}
	}

	ambiguous := map[string][]string{
		"chat +chat-list":    {"chat +my-groups", "chat +chat-list-mine", "chat +chat-list-all", "chat +conversation-list"},
		"chat +message-list": {"chat +chat-messages", "chat +messages-list-direct", "chat +search-msg", "chat +unread-chats"},
		"chat +send":         {"chat +messages-send", "chat +dm", "chat +send-to-group"},
		"chat +send-message": {"chat +messages-send", "chat +dm", "chat +send-to-group"},
		"chat +send-text":    {"chat +messages-send", "chat +dm", "chat +send-to-group"},
		"chat +send-to":      {"chat +messages-send", "chat +dm", "chat +send-to-group"},
		"chat +send-file":    {"chat +messages-send", "chat message send"},
		"chat +send-image":   {"chat +messages-send", "chat message send"},
		"chat +send-media":   {"chat +messages-send", "chat message send"},
		"oa +list-processes": {"oa +list-forms", "oa +my-initiated", "oa approval list-initiated"},
	}
	for from, candidates := range ambiguous {
		entry, ok := LookupCommandPathFallback(from)
		if !ok || entry.Mode != CommandPathFallbackAmbiguous || !reflect.DeepEqual(entry.Candidates, candidates) {
			t.Errorf("ambiguous %q = %#v, %v; want %v", from, entry, ok, candidates)
		}
	}

	for _, path := range []string{
		"chat +read-single",
		"chat +send-dm",
		"chat +send-single",
		"chat +group-send-text",
		"chat +send-by-bot",
		"chat +rename-group",
	} {
		if entry, ok := LookupCommandPathFallback(path); ok {
			t.Errorf("unsafe or contract-incomplete path %q unexpectedly falls back: %#v", path, entry)
		}
	}
}

func TestReduceCommandPathFallbacksValidatesLiveTree(t *testing.T) {
	root := commandFallbackTestRoot()
	entries := []CommandPathFallback{
		{From: "chat +bad", Mode: CommandPathFallbackRewrite, To: "chat +good", Reviewed: true, ReviewReason: "fixture"},
		{From: "chat group search", Mode: CommandPathFallbackRewrite, To: "chat search", Reviewed: true, ReviewReason: "fixture"},
		{From: "chat +choose", Mode: CommandPathFallbackAmbiguous, Candidates: []string{"chat +good", "chat +other"}, Reviewed: true, ReviewReason: "fixture"},
		{From: "chat +mixed", Mode: CommandPathFallbackAmbiguous, Candidates: []string{"chat +good", "chat search"}, Reviewed: true, ReviewReason: "fixture"},
	}
	withCommandFallbackSource(t, entries)
	got, err := ReduceCommandPathFallbacks(root)
	if err != nil || len(got) != len(entries) {
		t.Fatalf("ReduceCommandPathFallbacks() = %#v, %v", got, err)
	}
	got[2].Candidates[0] = "mutated"
	again, err := ReduceCommandPathFallbacks(root)
	if err != nil || again[2].Candidates[0] == "mutated" {
		t.Fatalf("reduction did not clone candidates: %#v, %v", again, err)
	}
}

func TestReduceCommandPathFallbacksRejectsUnsafeMappings(t *testing.T) {
	baseRoot := commandFallbackTestRoot
	tests := map[string]struct {
		entries []CommandPathFallback
		mutate  func(*cobra.Command)
		want    string
	}{
		"missing target": {
			entries: []CommandPathFallback{{From: "chat +bad", Mode: CommandPathFallbackRewrite, To: "chat +missing"}},
			want:    "does not exist",
		},
		"real source collision": {
			entries: []CommandPathFallback{{From: "chat +other", Mode: CommandPathFallbackRewrite, To: "chat +good"}},
			want:    "collides with a real Cobra command or alias",
		},
		"cobra alias collision": {
			entries: []CommandPathFallback{{From: "chat +official-alias", Mode: CommandPathFallbackRewrite, To: "chat +other"}},
			want:    "collides with a real Cobra command or alias",
		},
		"cross service": {
			entries: []CommandPathFallback{{From: "chat +bad", Mode: CommandPathFallbackRewrite, To: "mail +good"}},
			want:    "crosses service boundary",
		},
		"shortcut mismatch": {
			entries: []CommandPathFallback{{From: "chat +bad", Mode: CommandPathFallbackRewrite, To: "chat search"}},
			want:    "disagree on +shortcut identity",
		},
		"chained rewrite": {
			entries: []CommandPathFallback{
				{From: "chat +bad", Mode: CommandPathFallbackRewrite, To: "chat +next"},
				{From: "chat +next", Mode: CommandPathFallbackRewrite, To: "chat +good"},
			},
			want: "chained fallbacks are forbidden",
		},
		"missing ambiguous candidate": {
			entries: []CommandPathFallback{{From: "chat +choose", Mode: CommandPathFallbackAmbiguous, Candidates: []string{"chat +good", "chat +missing"}}},
			want:    "does not exist",
		},
		"hidden target": {
			entries: []CommandPathFallback{{From: "chat +bad", Mode: CommandPathFallbackRewrite, To: "chat +good"}},
			mutate: func(root *cobra.Command) {
				exactSchemaCommand(root, "chat +good").Hidden = true
			},
			want: "not a public runnable",
		},
		"target is alias": {
			entries: []CommandPathFallback{{From: "chat +bad", Mode: CommandPathFallbackRewrite, To: "chat +official-alias"}},
			want:    "must use canonical Cobra names",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			root := baseRoot()
			if test.mutate != nil {
				test.mutate(root)
			}
			for index := range test.entries {
				test.entries[index].Reviewed = true
				test.entries[index].ReviewReason = "fixture"
			}
			withCommandFallbackSource(t, test.entries)
			if _, err := ReduceCommandPathFallbacks(root); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("reduction error = %v, want containing %q", err, test.want)
			}
		})
	}

	withCommandFallbackLoadError(t, errors.New("fixture load"))
	if _, err := ReduceCommandPathFallbacks(baseRoot()); err == nil || !strings.Contains(err.Error(), "fixture load") {
		t.Fatalf("load error = %v", err)
	}
	if _, err := ReduceCommandPathFallbacks(nil); err == nil || !strings.Contains(err.Error(), "root is nil") {
		t.Fatalf("nil root error = %v", err)
	}
}

func commandFallbackTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "dws"}
	chat := &cobra.Command{Use: "chat"}
	good := &cobra.Command{Use: "+good", Aliases: []string{"+official-alias"}, Run: func(*cobra.Command, []string) {}}
	chat.AddCommand(
		good,
		&cobra.Command{Use: "+other", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "search", Run: func(*cobra.Command, []string) {}},
	)
	group := &cobra.Command{Use: "group"}
	group.AddCommand(cmdutil.HintSubCmd("search", "use chat search"))
	chat.AddCommand(group)
	mail := &cobra.Command{Use: "mail"}
	mail.AddCommand(&cobra.Command{Use: "+good", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(chat, mail)
	return root
}

func withCommandFallbackSource(t *testing.T, entries []CommandPathFallback) {
	t.Helper()
	old := loadReviewedCommandPathFallbacks
	loadReviewedCommandPathFallbacks = func() ([]CommandPathFallback, error) {
		return cloneCommandPathFallbacks(entries), nil
	}
	t.Cleanup(func() { loadReviewedCommandPathFallbacks = old })
}

func withCommandFallbackLoadError(t *testing.T, want error) {
	t.Helper()
	old := loadReviewedCommandPathFallbacks
	loadReviewedCommandPathFallbacks = func() ([]CommandPathFallback, error) { return nil, want }
	t.Cleanup(func() { loadReviewedCommandPathFallbacks = old })
}

func TestCommandPathFallbackGeneratedTableWellFormed(t *testing.T) {
	entries := loadGeneratedCommandPathFallbacks()
	if len(entries) == 0 {
		t.Fatal("generated command path fallback table is empty")
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if seen[entry.From] || !entry.Reviewed || strings.TrimSpace(entry.ReviewReason) == "" {
			t.Fatalf("invalid generated entry: %#v", entry)
		}
		seen[entry.From] = true
	}
	if !reflect.DeepEqual(entries, cloneCommandPathFallbacks(entries)) {
		t.Fatal("generated table clone changed values")
	}
}
