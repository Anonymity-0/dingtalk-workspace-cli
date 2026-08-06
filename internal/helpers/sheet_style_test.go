// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type sheetStyleDryRunCaller struct {
	format string
	calls  int
}

func (c *sheetStyleDryRunCaller) CallTool(context.Context, string, string, map[string]any) (*edition.ToolResult, error) {
	c.calls++
	return &edition.ToolResult{}, nil
}

func (c *sheetStyleDryRunCaller) Format() string { return c.format }
func (*sheetStyleDryRunCaller) DryRun() bool     { return true }
func (*sheetStyleDryRunCaller) Fields() string   { return "" }
func (*sheetStyleDryRunCaller) JQ() string       { return "" }

func TestCrossPlatformCoverageRangeBatchSetStyleDryRunNeverCallsRemote(t *testing.T) {
	previousDeps := deps
	t.Cleanup(func() { deps = previousDeps })

	batchPath := filepath.Join(t.TempDir(), "styles.json")
	if err := os.WriteFile(batchPath, []byte(`[{"sheetId":"Sheet1","range":"A1:B2","fontWeight":"bold"}]`), 0o600); err != nil {
		t.Fatalf("write batch fixture: %v", err)
	}

	for _, format := range []string{"table", "json"} {
		t.Run(format, func(t *testing.T) {
			caller := &sheetStyleDryRunCaller{format: format}
			InitDeps(caller)
			var output bytes.Buffer
			deps.Out.w = &output
			deps.Out.errW = &output

			cmd := newRangeBatchSetStyleCmd()
			cmd.SetArgs([]string{"--node", "NODE_ID", "--batch", batchPath})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("batch-set-style dry-run error: %v", err)
			}
			if caller.calls != 0 {
				t.Fatalf("remote CallTool count = %d, want 0", caller.calls)
			}
			preview := output.String()
			if format == "json" {
				var payload map[string]any
				if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
					t.Fatalf("JSON dry-run stdout must be one document: %v\n%s", err, preview)
				}
				// batch-set-style 现在组装为一次 batch_update 原子提交，
				// dry-run 因此是单条标准记录，而不是本地循环产生的 results 数组。
				if payload["tool"] != "batch_update" {
					t.Fatalf("JSON dry-run tool = %#v, want batch_update", payload["tool"])
				}
				args, _ := payload["arguments"].(map[string]any)
				ops, _ := args["operations"].([]any)
				if len(ops) != 1 {
					t.Fatalf("JSON dry-run operations = %#v, want 1 item", args["operations"])
				}
				op, _ := ops[0].(map[string]any)
				if op["toolName"] != "set_cell_range" {
					t.Fatalf("JSON dry-run operation toolName = %#v", op["toolName"])
				}
			} else {
				for _, want := range []string{"Tool:", "batch_update", "set_cell_range", "Arguments:"} {
					if !strings.Contains(preview, want) {
						t.Fatalf("dry-run preview missing %q:\n%s", want, preview)
					}
				}
			}
		})
	}
}

func TestRangeBatchSetStylePropagatesJSONWriteFailure(t *testing.T) {
	previousDeps := deps
	t.Cleanup(func() { deps = previousDeps })

	batchPath := filepath.Join(t.TempDir(), "styles.json")
	if err := os.WriteFile(batchPath, []byte(`[{"sheetId":"Sheet1","range":"A1:B2","fontWeight":"bold"}]`), 0o600); err != nil {
		t.Fatalf("write batch fixture: %v", err)
	}

	caller := &sheetStyleDryRunCaller{format: "json"}
	InitDeps(caller)
	deps.Out.w = forcedJSONWriteFailure{}

	cmd := newRangeBatchSetStyleCmd()
	cmd.SetArgs([]string{"--node", "NODE_ID", "--batch", batchPath})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "forced JSON output failure") {
		t.Fatalf("error = %v, want JSON write failure", err)
	}
	if caller.calls != 0 {
		t.Fatalf("remote CallTool count = %d, want 0", caller.calls)
	}
}
