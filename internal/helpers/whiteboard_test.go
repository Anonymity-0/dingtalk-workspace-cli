package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type whiteboardTestCall struct {
	server string
	tool   string
	args   map[string]any
}

type whiteboardTestCaller struct {
	dry      bool
	format   string
	response func(whiteboardTestCall, int) string
	calls    []whiteboardTestCall
}

func (c *whiteboardTestCaller) CallTool(_ context.Context, server, tool string, args map[string]any) (*edition.ToolResult, error) {
	call := whiteboardTestCall{server: server, tool: tool, args: args}
	c.calls = append(c.calls, call)
	text := `{}`
	if c.response != nil {
		text = c.response(call, len(c.calls)-1)
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: text}}}, nil
}

func (c *whiteboardTestCaller) Format() string { return c.format }
func (c *whiteboardTestCaller) DryRun() bool   { return c.dry }
func (*whiteboardTestCaller) Fields() string   { return "" }
func (*whiteboardTestCaller) JQ() string       { return "" }

func installWhiteboardTestCaller(t *testing.T, caller *whiteboardTestCaller) *bytes.Buffer {
	t.Helper()
	previous := deps
	InitDeps(caller)
	output := &bytes.Buffer{}
	deps.Out.w = output
	deps.Out.errW = &bytes.Buffer{}
	t.Cleanup(func() { deps = previous })
	return output
}

func TestWhiteboardQueryRoutesAndDecodesResultJSON(t *testing.T) {
	caller := &whiteboardTestCaller{
		format: "json",
		response: func(whiteboardTestCall, int) string {
			return `{"success":true,"resultJson":"{\"nodes\":[{\"type\":\"text\"}]}"}`
		},
	}
	output := installWhiteboardTestCaller(t, caller)
	cmd := newWhiteboardCommand()
	cmd.SetArgs([]string{"query", "--node", "doc-1", "--part-id", "part-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 1 || caller.calls[0].server != "whiteboard" || caller.calls[0].tool != whiteboardQueryTool {
		t.Fatalf("calls = %#v", caller.calls)
	}
	if caller.calls[0].args["nodeId"] != "doc-1" || caller.calls[0].args["partId"] != "part-1" {
		t.Fatalf("args = %#v", caller.calls[0].args)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatalf("output = %q: %v", output.String(), err)
	}
	if _, ok := payload["resultJson"].(map[string]any); !ok {
		t.Fatalf("resultJson was not decoded: %#v", payload)
	}
}

func TestWhiteboardUpdateValidatesSourceAndRequiresConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whiteboard.json")
	if err := os.WriteFile(path, []byte(`{"overwrite":false,"source":{"schemaVersion":"1.0","catalogVersion":"dml-v1","nodes":[{"id":"n1","type":"text"}]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	caller := &whiteboardTestCaller{format: "json"}
	installWhiteboardTestCaller(t, caller)

	cmd := newWhiteboardCommand()
	cmd.SetIn(strings.NewReader("no\n"))
	cmd.SetArgs([]string{"update", "--node", "doc-1", "--part-id", "part-1", "--source", path})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("err = %v, want cancellation", err)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("remote call happened before confirmation: %#v", caller.calls)
	}

	cmd = newWhiteboardCommand()
	cmd.SetArgs([]string{"update", "--node", "doc-1", "--part-id", "part-1", "--source", path, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != whiteboardUpdateTool {
		t.Fatalf("calls = %#v", caller.calls)
	}
	if caller.calls[0].args["mode"] != "append" || caller.calls[0].args["nodes"] != `[{"id":"n1","type":"text"}]` {
		t.Fatalf("args = %#v", caller.calls[0].args)
	}
}

func TestDocWhiteboardInsertBuildsCardAndReturnsPersistedPartID(t *testing.T) {
	var blockID string
	caller := &whiteboardTestCaller{
		format: "json",
		response: func(call whiteboardTestCall, index int) string {
			if index == 0 {
				var node []any
				if err := json.Unmarshal([]byte(call.args["jsonml"].(string)), &node); err != nil {
					t.Fatalf("jsonml: %v", err)
				}
				attrs := node[1].(map[string]any)
				blockID = attrs["uuid"].(string)
				return `{}`
			}
			jsonml := fmt.Sprintf(`["card",{"uuid":%q,"cardType":"hetu","metadata":{"id":"part-real"}}]`, blockID)
			encoded, _ := json.Marshal(jsonml)
			return fmt.Sprintf(`{"blocks":[{"blockId":%q,"jsonml":%s}]}`, blockID, encoded)
		},
	}
	output := installWhiteboardTestCaller(t, caller)
	previousDelays := whiteboardRetryDelays
	whiteboardRetryDelays = nil
	t.Cleanup(func() { whiteboardRetryDelays = previousDelays })

	cmd := newDocWhiteboardCommand()
	cmd.SetArgs([]string{"insert", "--node", "doc-1", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 2 || caller.calls[0].tool != "insert_document_block" || caller.calls[1].tool != "list_document_blocks" {
		t.Fatalf("calls = %#v", caller.calls)
	}
	if caller.calls[0].server != "doc" || caller.calls[1].server != "doc" {
		t.Fatalf("unexpected servers: %#v", caller.calls)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatalf("output = %q: %v", output.String(), err)
	}
	result, _ := payload["result"].(map[string]any)
	if result["whiteboardId"] != "part-real" {
		t.Fatalf("output = %#v", payload)
	}
}

func TestDocMediaUploadReturnsStableResourceContract(t *testing.T) {
	file := filepath.Join(t.TempDir(), "icon.svg")
	if err := os.WriteFile(file, []byte("<svg/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	caller := &whiteboardTestCaller{
		format: "json",
		response: func(whiteboardTestCall, int) string {
			return `{"uploadUrl":"https://upload.example.test/token","resourceId":"res-1","resourceUrl":"https://resource.example.test/icon.svg"}`
		},
	}
	output := installWhiteboardTestCaller(t, caller)
	previousPut := httpPutFile
	httpPutFile = func(context.Context, string, map[string]string, string, int64) error { return nil }
	t.Cleanup(func() { httpPutFile = previousPut })

	cmd := newDocCommand()
	cmd.SetArgs([]string{"media", "upload", "--node", "doc-1", "--file", file, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 1 || caller.calls[0].server != "doc" || caller.calls[0].tool != "get_doc_attachment_upload_info" {
		t.Fatalf("calls = %#v", caller.calls)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatalf("output = %q: %v", output.String(), err)
	}
	if strings.Contains(output.String(), "upload.example.test") || payload["resourceId"] != "res-1" {
		t.Fatalf("output = %#v", payload)
	}
}

func TestDocMediaUploadRedactsTemporaryURLFromUploadError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "icon.svg")
	if err := os.WriteFile(file, []byte("<svg/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	uploadURL := "https://upload.example.test/secret-token"
	caller := &whiteboardTestCaller{
		format: "json",
		response: func(whiteboardTestCall, int) string {
			return fmt.Sprintf(`{"uploadUrl":%q,"resourceId":"res-1","resourceUrl":"https://resource.example.test/icon.svg"}`, uploadURL)
		},
	}
	installWhiteboardTestCaller(t, caller)
	previousPut := httpPutFile
	httpPutFile = func(context.Context, string, map[string]string, string, int64) error {
		return fmt.Errorf("PUT %s: connection reset", uploadURL)
	}
	t.Cleanup(func() { httpPutFile = previousPut })

	cmd := newDocCommand()
	cmd.SetArgs([]string{"media", "upload", "--node", "doc-1", "--file", file, "--yes"})
	err := cmd.Execute()
	if err == nil || strings.Contains(err.Error(), uploadURL) || !strings.Contains(err.Error(), "<redacted upload URL>") {
		t.Fatalf("err = %v, want redacted temporary upload URL", err)
	}
}
