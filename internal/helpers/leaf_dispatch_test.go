// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

// leafDispatchCaller 是 leaf 默认派发路径的 fake ToolCaller。
type leafDispatchCaller struct {
	dryRun    bool
	productID string
	toolName  string
}

func (c *leafDispatchCaller) CallTool(_ context.Context, productID, toolName string, _ map[string]any) (*edition.ToolResult, error) {
	c.productID = productID
	c.toolName = toolName
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: `{}`}}}, nil
}

func (c *leafDispatchCaller) Format() string { return "json" }
func (c *leafDispatchCaller) DryRun() bool   { return c.dryRun }
func (c *leafDispatchCaller) Fields() string { return "" }
func (c *leafDispatchCaller) JQ() string     { return "" }

func withLeafDispatchCaller(t *testing.T, dryRun bool) *leafDispatchCaller {
	t.Helper()
	previousDeps := deps
	t.Cleanup(func() { deps = previousDeps })
	caller := &leafDispatchCaller{dryRun: dryRun}
	InitDeps(caller)
	deps.Out.w = io.Discard
	return caller
}

// TestLeafCommandDefaultDispatch 覆盖默认 callMCPTool 派发（无 Call/Server）。
// DryRun caller 在路由前早返回，验证 RunE 走到默认派发语句且无错误。
func TestLeafCommandDefaultDispatch(t *testing.T) {
	withLeafDispatchCaller(t, true)
	cmd := NewLeafCommand(LeafSpec{
		Use: "get", Tool: "get_thing",
		Flags: []LeafFlag{{Name: "id", Usage: "ID", Bind: "id"}},
	})
	if err := cmd.Flags().Set("id", "x"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() = %v, want nil via dry-run dispatch", err)
	}
}

// TestLeafCommandServerDispatch 覆盖显式 Server 路由分支。
func TestLeafCommandServerDispatch(t *testing.T) {
	caller := withLeafDispatchCaller(t, false)
	cmd := NewLeafCommand(LeafSpec{
		Use: "get", Server: "doc", Tool: "get_document",
		Flags: []LeafFlag{{Name: "id", Usage: "ID", Bind: "docId"}},
	})
	if err := cmd.Flags().Set("id", "d1"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() = %v", err)
	}
	if caller.productID != "doc" || caller.toolName != "get_document" {
		t.Fatalf("dispatched to %s/%s, want doc/get_document", caller.productID, caller.toolName)
	}
}

// TestLeafCommandTransformErrorAborts 覆盖 RunE 中 leafArgs 错误传播。
func TestLeafCommandTransformErrorAborts(t *testing.T) {
	withLeafDispatchCaller(t, true)
	cmd := NewLeafCommand(LeafSpec{
		Use: "get", Tool: "get_thing",
		Flags: []LeafFlag{{
			Name: "when", Usage: "时间", Bind: "when",
			Transform: func(string) (any, error) { return nil, errors.New("bad time format") },
		}},
	})
	if err := cmd.Flags().Set("when", "not-a-time"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err == nil || !strings.Contains(err.Error(), "bad time format") {
		t.Fatalf("RunE() = %v, want transform error", err)
	}
}

// TestLeafValidateRequiredEnvDefaultHint 覆盖 EnvVar Required 未配置
// RequiredHint 时的默认报错文案。
func TestLeafValidateRequiredEnvDefaultHint(t *testing.T) {
	spec := LeafSpec{
		Use: "send", Tool: "send_thing",
		Flags: []LeafFlag{{Name: "token", Usage: "令牌", Required: true, EnvVar: "DWS_LEAF_TEST_HINTLESS"}},
	}
	cmd := NewLeafCommand(spec)
	err := leafValidateRequired(cmd, spec)
	if err == nil || err.Error() != "flag --token is required" {
		t.Fatalf("leafValidateRequired() = %v, want default hint", err)
	}
}
