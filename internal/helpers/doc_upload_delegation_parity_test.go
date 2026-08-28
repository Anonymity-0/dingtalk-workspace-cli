// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

// docUploadParityInner scripts the doc-space three-step upload backend
// (check_capability -> get_file_upload_info -> commit_uploaded_file) and
// records every call in order so tests can assert the FIRST delegated call
// carries the operation-level options built from the shared step1Args.
type docUploadParityInner struct {
	calls        []docDelegationCall
	checkResText string
}

func (c *docUploadParityInner) CallTool(_ context.Context, server, tool string, args map[string]any) (*edition.ToolResult, error) {
	copied := map[string]any{}
	for k, v := range args {
		copied[k] = v
	}
	c.calls = append(c.calls, docDelegationCall{server: server, tool: tool, args: copied})
	switch tool {
	case checkCapTool:
		return textToolResult(c.checkResText), nil
	case "get_file_upload_info":
		return textToolResult(`{"resourceUrl":"https://upload.example.test/object","uploadKey":"key-1"}`), nil
	case "commit_uploaded_file":
		return textToolResult(`{"dentryUuid":"node-1","name":"x.md"}`), nil
	}
	return textToolResult(`{"ok":true}`), nil
}

func (*docUploadParityInner) Format() string { return "json" }
func (*docUploadParityInner) DryRun() bool   { return false }
func (*docUploadParityInner) Fields() string { return "" }
func (*docUploadParityInner) JQ() string     { return "" }

// runDocUploadRealPath drives the non-dry `doc upload` command against the
// supplied caller and reports whether the HTTP PUT was reached. The real
// execution path is exercised end to end (get_file_upload_info -> PUT ->
// commit_uploaded_file) so the FIRST get_file_upload_info args can be asserted.
func runDocUploadRealPath(t *testing.T, caller edition.ToolCaller, folder, name string) (putReached bool, err error) {
	t.Helper()
	prevDeps := deps
	prevArgs := os.Args
	t.Cleanup(func() {
		deps = prevDeps
		os.Args = prevArgs
		SetHTTPPutFile(nil)
	})
	InitDeps(caller)
	deps.Out.w = io.Discard
	deps.Out.errW = io.Discard
	os.Args = []string{"dws", "doc"}
	SetHTTPPutFile(func(context.Context, string, map[string]string, string, int64) error {
		putReached = true
		return nil
	})

	file := filepath.Join(t.TempDir(), "src.md")
	if writeErr := os.WriteFile(file, []byte("hello-body"), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	root := newDocCommand()
	cmd, _, findErr := root.Find([]string{"upload"})
	if findErr != nil {
		t.Fatalf("find upload command: %v", findErr)
	}
	_ = cmd.Flags().Set("file", file)
	if folder != "" {
		_ = cmd.Flags().Set("folder", folder)
	}
	if name != "" {
		_ = cmd.Flags().Set("name", name)
	}
	return putReached, cmd.RunE(cmd, nil)
}

// TestCrossPlatformCoverageDocUploadStep1ArgsSharedConstructor pins the single
// source of truth docFileUploadInfoArgs: fileSize is unconditional, name is set
// when non-empty, and overwriteNodeId (when present) supersedes folderId. Both
// the folder and overwrite shapes carry name+fileSize so the first capability
// check always has an operation-level uploadActionParam.
func TestCrossPlatformCoverageDocUploadStep1ArgsSharedConstructor(t *testing.T) {
	folder := docFileUploadInfoArgs("x.md", 12, "f1", "w1", "")
	if folder["name"] != "x.md" {
		t.Fatalf("folder mode name = %v, want x.md", folder["name"])
	}
	if folder["fileSize"] != float64(12) {
		t.Fatalf("folder mode fileSize = %v (%T), want float64(12)", folder["fileSize"], folder["fileSize"])
	}
	if folder["folderId"] != "f1" || folder["workspaceId"] != "w1" {
		t.Fatalf("folder mode target = %#v, want folderId=f1 workspaceId=w1", folder)
	}
	if _, has := folder["overwriteNodeId"]; has {
		t.Fatalf("folder mode must not carry overwriteNodeId: %#v", folder)
	}

	overwrite := docFileUploadInfoArgs("x.md", 34, "f1", "w1", "node-9")
	if overwrite["name"] != "x.md" || overwrite["fileSize"] != float64(34) {
		t.Fatalf("overwrite mode name/size = %#v, want name=x.md fileSize=34", overwrite)
	}
	if overwrite["overwriteNodeId"] != "node-9" {
		t.Fatalf("overwrite mode overwriteNodeId = %v, want node-9", overwrite["overwriteNodeId"])
	}
	if _, has := overwrite["folderId"]; has {
		t.Fatalf("overwrite mode must exclude folderId (mutually exclusive): %#v", overwrite)
	}

	if _, has := docFileUploadInfoArgs("", 0, "", "", "")["name"]; has {
		t.Fatal("empty name must be omitted")
	}
}

// TestCrossPlatformCoverageDocUploadRealFirstCallCarriesNameSize proves the
// standard `doc upload` real execution issues get_file_upload_info as its first
// call already carrying name+fileSize (previously only folderId/workspaceId).
func TestCrossPlatformCoverageDocUploadRealFirstCallCarriesNameSize(t *testing.T) {
	caller := &scriptedToolCaller{steps: []scriptedToolStep{
		{text: `{"resourceUrl":"https://upload.example.test/object","uploadKey":"key-1"}`},
		{text: `{"ok":true}`},
	}}
	putReached, err := runDocUploadRealPath(t, caller, "f1", "renamed.md")
	if err != nil {
		t.Fatalf("doc upload real path error = %v", err)
	}
	if !putReached {
		t.Fatal("expected HTTP PUT to be reached on the allow/no-delegation path")
	}
	if len(caller.toolLog) == 0 || caller.toolLog[0] != "get_file_upload_info" {
		t.Fatalf("first tool = %#v, want get_file_upload_info", caller.toolLog)
	}
	first := caller.argsLog[0]
	if first["name"] != "renamed.md" {
		t.Fatalf("first get_file_upload_info name = %v, want renamed.md (regression: name dropped)", first["name"])
	}
	if first["fileSize"] != float64(len("hello-body")) {
		t.Fatalf("first get_file_upload_info fileSize = %v, want %d", first["fileSize"], len("hello-body"))
	}
	if first["folderId"] != "f1" {
		t.Fatalf("first get_file_upload_info folderId = %v, want f1", first["folderId"])
	}
}

// TestCrossPlatformCoverageDocImportUploadFallbackFirstCallCarriesNameSize
// covers the import upload fallback (docSpaceUploadCommitText): its first real
// get_file_upload_info must also carry name+fileSize so the fallback authorizes
// precisely before streaming bytes.
func TestCrossPlatformCoverageDocImportUploadFallbackFirstCallCarriesNameSize(t *testing.T) {
	caller := &docImportTargetCaller{responses: map[string][]scriptedToolStep{
		"get_file_upload_info": {{text: `{"resourceUrl":"https://upload.example.test/object","uploadKey":"key-1"}`}},
		"commit_uploaded_file": {{text: `{"dentryUuid":"node-1","name":"page.html"}`}},
	}}
	if _, err := runDocImportTargetFlow(t, caller, "html", "folder-1", ""); err != nil {
		t.Fatalf("import upload fallback error = %v", err)
	}
	if len(caller.calls) == 0 || caller.calls[0].tool != "get_file_upload_info" {
		t.Fatalf("first fallback call = %#v, want get_file_upload_info", caller.calls)
	}
	first := caller.calls[0].args
	if name, ok := first["name"].(string); !ok || name == "" {
		t.Fatalf("fallback get_file_upload_info name = %#v, want non-empty (regression: name dropped)", first["name"])
	}
	if size, ok := first["fileSize"].(float64); !ok || size <= 0 {
		t.Fatalf("fallback get_file_upload_info fileSize = %#v, want positive float64", first["fileSize"])
	}
}

// TestCrossPlatformCoverageDocUploadDelegationDeniedBeforePut is the core
// auto-CR guard: when the principal is denied, the very first delegated call
// (check_capability for get_file_upload_info) already carries the
// uploadActionParam operation options and the command fails BEFORE any HTTP PUT
// or commit_uploaded_file — no orphaned object is left behind.
func TestCrossPlatformCoverageDocUploadDelegationDeniedBeforePut(t *testing.T) {
	inner := &docUploadParityInner{checkResText: `{"allowed":false,"denialReason":"NO_PERM","denialMessage":"没有该文档的委托权限"}`}
	decorator := newDocDelegationAuthDecorator(inner)

	putReached, err := runDocUploadRealPath(t, decorator, "f1", "x.md")
	if err == nil {
		t.Fatal("expected delegation denial error, got nil")
	}
	if !strings.HasPrefix(err.Error(), "[DELEGATION_AUTH_DENIED]") {
		t.Fatalf("Error() = %q, want [DELEGATION_AUTH_DENIED] prefix", err.Error())
	}
	if putReached {
		t.Fatal("HTTP PUT reached before authorization: rejection must happen before uploading bytes")
	}
	if len(inner.calls) != 1 {
		t.Fatalf("inner calls = %#v, want only the check_capability call (no upload/commit passthrough)", inner.calls)
	}
	check := inner.calls[0]
	if check.tool != checkCapTool || check.args["mcpToolKey"] != "doc.get_file_upload_info" {
		t.Fatalf("first call = %#v, want check_capability for doc.get_file_upload_info", check)
	}
	assertUploadActionParam(t, check.args, "x.md")
}

// TestCrossPlatformCoverageDocUploadDelegationAllowedFirstCallMatches verifies
// the allow path: the first precise tool call is exactly the shared step1Args
// (name+fileSize+folderId) and its capability check carried the matching
// uploadActionParam, i.e. precheck options == real first-call options.
func TestCrossPlatformCoverageDocUploadDelegationAllowedFirstCallMatches(t *testing.T) {
	inner := &docUploadParityInner{checkResText: `{"allowed":true}`}
	decorator := newDocDelegationAuthDecorator(inner)

	putReached, err := runDocUploadRealPath(t, decorator, "f1", "x.md")
	if err != nil {
		t.Fatalf("doc upload allow path error = %v", err)
	}
	if !putReached {
		t.Fatal("expected HTTP PUT to run after the capability check allowed the principal")
	}
	if len(inner.calls) < 2 {
		t.Fatalf("inner calls = %#v, want check followed by get_file_upload_info passthrough", inner.calls)
	}
	check := inner.calls[0]
	if check.tool != checkCapTool || check.args["mcpToolKey"] != "doc.get_file_upload_info" {
		t.Fatalf("call[0] = %#v, want check_capability for doc.get_file_upload_info", check)
	}
	assertUploadActionParam(t, check.args, "x.md")

	first := inner.calls[1]
	if first.tool != "get_file_upload_info" {
		t.Fatalf("call[1] = %#v, want get_file_upload_info passthrough", first)
	}
	if first.args["name"] != "x.md" || first.args["fileSize"] != float64(len("hello-body")) || first.args["folderId"] != "f1" {
		t.Fatalf("first precise call args = %#v, want name=x.md fileSize=%d folderId=f1", first.args, len("hello-body"))
	}
}

// assertUploadActionParam checks the capability check options carry the
// operation-level uploadActionParam with the given fileName and a fileSize.
func assertUploadActionParam(t *testing.T, checkArgs map[string]any, wantFileName string) {
	t.Helper()
	opts, ok := checkArgs["options"].(map[string]any)
	if !ok {
		t.Fatalf("check options = %#v, want operation-level options map", checkArgs["options"])
	}
	param, ok := opts["uploadActionParam"].(map[string]any)
	if !ok {
		t.Fatalf("options = %#v, want uploadActionParam", opts)
	}
	if param["fileName"] != wantFileName {
		t.Fatalf("uploadActionParam.fileName = %v, want %s", param["fileName"], wantFileName)
	}
	if _, has := param["fileSize"]; !has {
		t.Fatalf("uploadActionParam = %#v, want fileSize present", param)
	}
}
