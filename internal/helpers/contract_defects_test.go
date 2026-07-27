// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
	"github.com/spf13/cobra"
)

type contractDefectCaller struct {
	dryRun    bool
	calls     []guardedMutationCall
	responses map[string]string
	errors    map[string]error
}

func (c *contractDefectCaller) CallTool(_ context.Context, productID, toolName string, args map[string]any) (*edition.ToolResult, error) {
	c.calls = append(c.calls, guardedMutationCall{productID: productID, toolName: toolName, args: args})
	key := productID + "/" + toolName
	if err := c.errors[key]; err != nil {
		return nil, err
	}
	text := c.responses[key]
	if text == "" {
		text = `{}`
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: text}}}, nil
}

func (*contractDefectCaller) Format() string { return "json" }
func (c *contractDefectCaller) DryRun() bool { return c.dryRun }
func (*contractDefectCaller) Fields() string { return "" }
func (*contractDefectCaller) JQ() string     { return "" }

func executeContractDefectCommand(t *testing.T, caller *contractDefectCaller, build func() *cobra.Command, args ...string) (string, error) {
	t.Helper()
	previousDeps := deps
	t.Cleanup(func() { deps = previousDeps })

	InitDeps(caller)
	var stdout bytes.Buffer
	deps.Out.w = &stdout
	deps.Out.errW = io.Discard

	root := build()
	if root.PersistentFlags().Lookup("yes") == nil {
		root.PersistentFlags().Bool("yes", false, "confirm high-risk operation")
	}
	if root.PersistentFlags().Lookup("dry-run") == nil {
		root.PersistentFlags().Bool("dry-run", false, "preview without executing")
	}
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), err
}

func TestApprovalRevokeDryRunSkipsConfirmationAndEmitsPreview(t *testing.T) {
	caller := &contractDefectCaller{dryRun: true}
	output, err := executeContractDefectCommand(t, caller, newOaCommand,
		"approval", "revoke", "--instance-id", "instance-dry-run", "--dry-run")
	if err != nil {
		t.Fatalf("approval revoke dry-run returned error: %v", err)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("dry-run tool calls = %#v, want none", caller.calls)
	}
	if !strings.Contains(output, `"tool": "revoke_processInstance"`) ||
		!strings.Contains(output, `"processInstanceId": "instance-dry-run"`) {
		t.Fatalf("dry-run output = %q, want revoke preview", output)
	}
}

func TestDocVersionRevertDryRunSkipsRemotePreflightAndEmitsPreview(t *testing.T) {
	caller := &contractDefectCaller{dryRun: true}
	output, err := executeContractDefectCommand(t, caller, newDocCommand,
		"version", "revert", "--node", "node-dry-run", "--version", "7", "--dry-run")
	if err != nil {
		t.Fatalf("doc version revert dry-run returned error: %v", err)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("dry-run tool calls = %#v, want none", caller.calls)
	}
	if !strings.Contains(output, `"tool": "revert_doc_version"`) ||
		!strings.Contains(output, `"version": 7`) {
		t.Fatalf("dry-run output = %q, want version revert preview", output)
	}
}

func TestRenameDocumentRemovesOnePreservedExtension(t *testing.T) {
	tests := []struct {
		name  string
		build func() *cobra.Command
		args  []string
	}{
		{
			name:  "doc",
			build: newDocCommand,
			args:  []string{"rename", "--node", "node-1", "--name", "report.txt"},
		},
		{
			name:  "drive",
			build: newDriveCommand,
			args:  []string{"rename", "--node", "node-1", "--name", "report.txt"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &contractDefectCaller{}
			if _, err := executeContractDefectCommand(t, caller, test.build, test.args...); err != nil {
				t.Fatalf("rename returned error: %v", err)
			}
			if len(caller.calls) != 1 {
				t.Fatalf("tool calls = %#v, want one rename call", caller.calls)
			}
			if got := caller.calls[0].args["newName"]; got != "report" {
				t.Fatalf("newName = %#v, want report", got)
			}
		})
	}

	if got := renameBaseName("release.v2"); got != "release.v2" {
		t.Fatalf("unknown dotted suffix = %q, want release.v2", got)
	}
	if got := renameBaseName("REPORT.XLSX"); got != "REPORT" {
		t.Fatalf("case-insensitive extension normalization = %q, want REPORT", got)
	}
}

func TestDocInfoEnrichesMissingFileSizeFromDriveMetadata(t *testing.T) {
	caller := &contractDefectCaller{
		responses: map[string]string{
			"doc/get_document_info": `{"result":{"title":"budget.xlsx","fileSize":null}}`,
			"drive/get_file_info":   `{"result":{"fileSize":93682}}`,
		},
	}
	output, err := executeContractDefectCommand(t, caller, newDocCommand,
		"info", "--node", "node-1")
	if err != nil {
		t.Fatalf("doc info returned error: %v", err)
	}
	wantCalls := []guardedMutationCall{
		{productID: "doc", toolName: "get_document_info", args: map[string]any{"nodeId": "node-1"}},
		{productID: "drive", toolName: "get_file_info", args: map[string]any{"fileId": "node-1"}},
	}
	if !reflect.DeepEqual(caller.calls, wantCalls) {
		t.Fatalf("tool calls = %#v, want %#v", caller.calls, wantCalls)
	}
	var payload struct {
		Result struct {
			FileSize int64 `json:"fileSize"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("decode doc info output %q: %v", output, err)
	}
	if payload.Result.FileSize != 93682 {
		t.Fatalf("fileSize = %d, want 93682", payload.Result.FileSize)
	}
}

func TestDocInfoFallsBackWhenDriveMetadataIsUnavailable(t *testing.T) {
	caller := &contractDefectCaller{
		responses: map[string]string{
			"doc/get_document_info": `{"result":{"title":"online document"}}`,
		},
		errors: map[string]error{
			"drive/get_file_info": fmt.Errorf("drive metadata unavailable"),
		},
	}
	output, err := executeContractDefectCommand(t, caller, newDocCommand,
		"info", "--node", "node-1")
	if err != nil {
		t.Fatalf("doc info fallback returned error: %v", err)
	}
	if !strings.Contains(output, `"title": "online document"`) {
		t.Fatalf("fallback output = %q, want original document metadata", output)
	}
}

func TestDocInfoDryRunUsesSingleStandardPreview(t *testing.T) {
	caller := &contractDefectCaller{dryRun: true}
	output, err := executeContractDefectCommand(t, caller, newDocCommand,
		"info", "--node", "node-dry-run", "--dry-run")
	if err != nil {
		t.Fatalf("doc info dry-run returned error: %v", err)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("dry-run tool calls = %#v, want none", caller.calls)
	}
	if !strings.Contains(output, `"tool": "get_document_info"`) ||
		!strings.Contains(output, `"nodeId": "node-dry-run"`) {
		t.Fatalf("dry-run output = %q, want document info preview", output)
	}
}
