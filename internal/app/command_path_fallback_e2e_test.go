// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package app

import (
	"bytes"
	stderrors "errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/pipeline"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func TestReviewedCommandFallbacksReachCanonicalDryRunPayload(t *testing.T) {
	_, canonicalQuery, canonicalQueryAttempts, err := executeParamAliasDryRunE2E(t,
		"chat", "+chat-search", "--query", "project", "--dry-run",
	)
	if err != nil || len(canonicalQueryAttempts) != 0 {
		t.Fatalf("canonical query preview = %#v, attempts=%v, error=%v", canonicalQuery, canonicalQueryAttempts, err)
	}
	_, canonicalKeyword, canonicalKeywordAttempts, err := executeParamAliasDryRunE2E(t,
		"chat", "+chat-search", "--keyword", "project", "--dry-run",
	)
	if err != nil || len(canonicalKeywordAttempts) != 0 {
		t.Fatalf("canonical keyword preview = %#v, attempts=%v, error=%v", canonicalKeyword, canonicalKeywordAttempts, err)
	}
	_, nativeCanonical, nativeCanonicalAttempts, err := executeParamAliasDryRunE2E(t,
		"chat", "search", "--query", "project", "--dry-run",
	)
	if err != nil || len(nativeCanonicalAttempts) != 0 {
		t.Fatalf("native canonical preview = %#v, attempts=%v, error=%v", nativeCanonical, nativeCanonicalAttempts, err)
	}

	tests := []struct {
		name        string
		args        []string
		wantCommand string
		wantPreview paramAliasDryRunPreview
	}{
		{
			name:        "search group keyword",
			args:        []string{"chat", "+search-group", "--keyword", "project", "--dry-run"},
			wantCommand: "dws chat +chat-search",
			wantPreview: canonicalKeyword,
		},
		{
			name:        "group search query",
			args:        []string{"chat", "+group-search", "--query", "project", "--dry-run"},
			wantCommand: "dws chat +chat-search",
			wantPreview: canonicalQuery,
		},
		{
			name:        "chat group search query",
			args:        []string{"chat", "+chat-group-search", "--query", "project", "--dry-run"},
			wantCommand: "dws chat +chat-search",
			wantPreview: canonicalQuery,
		},
		{
			name:        "native group search",
			args:        []string{"chat", "group", "search", "--query", "project", "--dry-run"},
			wantCommand: "dws chat search",
			wantPreview: nativeCanonical,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, preview, attempts, executeErr := executeParamAliasDryRunE2E(t, test.args...)
			if executeErr != nil {
				t.Fatalf("fallback execute error = %v", executeErr)
			}
			if len(attempts) != 0 {
				t.Fatalf("fallback crossed dry-run dispatch boundary: %#v", attempts)
			}
			if !reflect.DeepEqual(preview, test.wantPreview) {
				t.Fatalf("fallback preview = %#v, want canonical %#v", preview, test.wantPreview)
			}
			if ctx == nil || ctx.Command != test.wantCommand || len(ctx.Corrections) == 0 {
				t.Fatalf("fallback context = %#v, want command %q", ctx, test.wantCommand)
			}
			correction := ctx.Corrections[0]
			if correction.Handler != "command-path-fallback" || correction.Kind != "reviewed-fallback" {
				t.Fatalf("fallback correction = %#v", correction)
			}
		})
	}
}

func TestReviewedMemberFallbackResolvesCanonicalLeafBeforeParameterValidation(t *testing.T) {
	root := NewSchemaSourceRootCommand()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	args := []string{
		"chat", "+chat-group-members",
		"--conversation-id", "cid-fixture",
		"--member-types", "user",
	}
	root.SetArgs(args)
	ctx, err := pipeline.RunPreParseArgs(root, newPipelineEngine(), args)
	if err != nil {
		t.Fatal(err)
	}
	if ctx == nil || ctx.Command != "dws chat +chat-members-list" || len(ctx.Corrections) == 0 {
		t.Fatalf("member fallback context = %#v", ctx)
	}
	wantPrefix := []string{"chat", "+chat-members-list"}
	if len(ctx.Args) < len(wantPrefix) || !reflect.DeepEqual(ctx.Args[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("member fallback args = %v", ctx.Args)
	}
}

func TestReviewedAmbiguousCommandFallbackNeverDispatches(t *testing.T) {
	caller := &paramAliasCaptureCaller{}
	ctx, err := executeParamAliasE2E(t, caller, "chat", "+chat-list", "--format", "json")
	if ctx == nil {
		t.Fatal("ambiguous command fallback returned nil context")
	}
	var structured *apperrors.Error
	if !stderrors.As(err, &structured) || structured.Reason != "ambiguous_command_fallback" || structured.ExitCode() != 3 {
		t.Fatalf("ambiguous error = %T %#v", err, err)
	}
	if len(structured.Actions) != 4 || len(caller.calls) != 0 {
		t.Fatalf("ambiguous actions=%v calls=%#v", structured.Actions, caller.calls)
	}
}

func TestCanonicalShortcutBadFlagDoesNotEnterCommandFallback(t *testing.T) {
	root := NewSchemaSourceRootCommand()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	args := []string{"chat", "+chat-search", "--definitely-not-a-real-flag", "project"}
	root.SetArgs(args)
	ctx, err := pipeline.RunPreParseArgs(root, newPipelineEngine(), args)
	if err != nil {
		t.Fatalf("preparse error = %v", err)
	}
	if ctx == nil || ctx.Command != "dws chat +chat-search" {
		t.Fatalf("canonical context = %#v", ctx)
	}
	for _, correction := range ctx.Corrections {
		if correction.Handler == "command-path-fallback" {
			t.Fatalf("canonical command received fallback correction: %#v", ctx.Corrections)
		}
	}
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("canonical bad flag error = %v", err)
	}
}

func TestRewrittenShortcutStillUsesCanonicalParameterErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing required query",
			args: []string{"chat", "+search-group", "--dry-run"},
			want: "请至少指定 --query、--keyword 之一",
		},
		{
			name: "unknown canonical flag",
			args: []string{"chat", "+search-group", "--definitely-not-a-real-flag", "project"},
			want: "unknown flag",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := NewSchemaSourceRootCommand()
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			root.SetArgs(test.args)
			ctx, err := pipeline.RunPreParseArgs(root, newPipelineEngine(), test.args)
			if err != nil {
				t.Fatalf("preparse error = %v", err)
			}
			if ctx == nil || ctx.Command != "dws chat +chat-search" || len(ctx.Corrections) == 0 ||
				ctx.Corrections[0].Handler != "command-path-fallback" {
				t.Fatalf("rewritten context = %#v", ctx)
			}
			if err := root.Execute(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("canonical parameter error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestCommandFallbackNamesStayOutOfHelpSchemaAndShortcutCatalog(t *testing.T) {
	invalidShortcuts := map[string]bool{
		"+search-group":       true,
		"+group-search":       true,
		"+chat-group-search":  true,
		"+chat-group-members": true,
		"+chat-list":          true,
	}
	root := NewSchemaSourceRootCommand()
	chat := exactAppCommand(root, "chat")
	if chat == nil {
		t.Fatal("chat command missing")
	}
	for _, child := range chat.Commands() {
		if invalidShortcuts[child.Name()] {
			t.Fatalf("fallback name %q became a real command", child.Name())
		}
		for _, alias := range child.Aliases {
			if invalidShortcuts[alias] {
				t.Fatalf("fallback name %q became a Cobra alias", alias)
			}
		}
	}
	for path := range invalidShortcuts {
		if _, ok := cli.ResolveMeta("chat " + path); ok {
			t.Fatalf("fallback path %q leaked into embedded Schema", path)
		}
	}
	for _, declared := range shortcut.All() {
		if declared.Service == "chat" && invalidShortcuts[declared.Command] {
			t.Fatalf("fallback name %q leaked into shortcut catalog", declared.Command)
		}
	}

	group := exactAppCommand(root, "chat group")
	searchHint := exactAppCommand(root, "chat group search")
	if group == nil || searchHint == nil || !searchHint.Hidden || !cmdutil.IsHintOnlyCommand(searchHint) {
		t.Fatalf("chat group search must remain a hidden hint-only node: group=%v search=%v", group, searchHint)
	}
	var help bytes.Buffer
	group.SetOut(&help)
	if err := group.Help(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(help.String(), "\n  search ") {
		t.Fatalf("hidden fallback source leaked into group help:\n%s", help.String())
	}
}

func exactAppCommand(root *cobra.Command, path string) *cobra.Command {
	current := root
	parts := strings.Fields(path)
	if len(parts) > 0 && parts[0] == root.Name() {
		parts = parts[1:]
	}
	for _, part := range parts {
		var next *cobra.Command
		for _, child := range current.Commands() {
			if child.Name() == part {
				next = child
				break
			}
		}
		if next == nil {
			return nil
		}
		current = next
	}
	return current
}
