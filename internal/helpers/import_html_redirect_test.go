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
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func htmlRedirectCommand(t *testing.T, filePath string) *cobra.Command {
	t.Helper()
	// callMCPToolReturnText 从 os.Args 解析产品名（doc）
	oldArgs := os.Args
	os.Args = []string{"dws", "doc"}
	t.Cleanup(func() { os.Args = oldArgs })
	cmd := &cobra.Command{Use: "import"}
	cmd.Flags().String("file", "", "")
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("folder", "", "")
	cmd.Flags().String("workspace", "", "")
	cmd.Flags().String("folder-id", "", "")
	cmd.Flags().String("workspace-id", "", "")
	cmd.Flags().Bool("convert", false, "")
	if filePath != "" {
		if err := cmd.Flags().Set("file", filePath); err != nil {
			t.Fatalf("set import file: %v", err)
		}
	}
	return cmd
}

func TestCrossPlatformCoverageDocImportHTMLUploadRedirect(t *testing.T) {
	uploadSteps := []scriptedToolStep{
		{text: `{"resourceUrl":"https://upload.example.test/object","uploadKey":"key-1"}`},
		{text: `{"dentryUuid":"node-1","name":"page.html"}`},
	}

	t.Run("html file is redirected to the doc upload chain", func(t *testing.T) {
		caller := &scriptedToolCaller{steps: uploadSteps}
		installScriptedCaller(t, caller)
		var warnings bytes.Buffer
		deps.Out.errW = &warnings
		SetHTTPPutFile(func(context.Context, string, map[string]string, string, int64) error { return nil })
		t.Cleanup(func() { SetHTTPPutFile(nil) })

		cmd := htmlRedirectCommand(t, writeImportFixture(t, "html"))
		if err := cmd.Flags().Set("workspace", "ws-1"); err != nil {
			t.Fatal(err)
		}
		if err := runImportCommand(cmd, nil, docImportFlowConfig()); err != nil {
			t.Fatalf("runImportCommand() error = %v, want upload redirect success", err)
		}
		if caller.calls != 2 || caller.tool != "commit_uploaded_file" {
			t.Fatalf("redirect calls = %d last tool = %q, want 2 calls ending in commit_uploaded_file", caller.calls, caller.tool)
		}
		if got := caller.args["workspaceId"]; got != "ws-1" {
			t.Fatalf("commit workspaceId = %v, want ws-1", got)
		}
		if !strings.Contains(warnings.String(), "文件上传链路") {
			t.Fatalf("redirect must announce the upload fallback on stderr, got %q", warnings.String())
		}
	})

	t.Run("uppercase htm extension via positional argument", func(t *testing.T) {
		caller := &scriptedToolCaller{steps: uploadSteps}
		installScriptedCaller(t, caller)
		SetHTTPPutFile(func(context.Context, string, map[string]string, string, int64) error { return nil })
		t.Cleanup(func() { SetHTTPPutFile(nil) })

		path := writeImportFixture(t, "HTM")
		cmd := htmlRedirectCommand(t, "")
		if err := runImportCommand(cmd, []string{path}, docImportFlowConfig()); err != nil {
			t.Fatalf("runImportCommand() error = %v, want upload redirect success", err)
		}
		if caller.tool != "commit_uploaded_file" {
			t.Fatalf("last tool = %q, want commit_uploaded_file", caller.tool)
		}
		if got, _ := cmd.Flags().GetString("file"); got != path {
			t.Fatalf("positional file must be backfilled into --file, got %q", got)
		}
	})

	t.Run("hidden folder-id alias is normalized for the upload chain", func(t *testing.T) {
		caller := &scriptedToolCaller{steps: uploadSteps}
		installScriptedCaller(t, caller)
		SetHTTPPutFile(func(context.Context, string, map[string]string, string, int64) error { return nil })
		t.Cleanup(func() { SetHTTPPutFile(nil) })

		cmd := htmlRedirectCommand(t, writeImportFixture(t, "html"))
		if err := cmd.Flags().Set("folder-id", "folder-abc"); err != nil {
			t.Fatal(err)
		}
		if err := runImportCommand(cmd, nil, docImportFlowConfig()); err != nil {
			t.Fatalf("runImportCommand() error = %v, want upload redirect success", err)
		}
		if got := caller.args["folderId"]; got != "folder-abc" {
			t.Fatalf("commit folderId = %v, want folder-abc from --folder-id alias", got)
		}
	})

	t.Run("any non-importable format is redirected, not enumerated", func(t *testing.T) {
		for _, ext := range []string{"pdf", "zip", "png", "mp4"} {
			caller := &scriptedToolCaller{steps: uploadSteps}
			installScriptedCaller(t, caller)
			SetHTTPPutFile(func(context.Context, string, map[string]string, string, int64) error { return nil })
			t.Cleanup(func() { SetHTTPPutFile(nil) })

			cmd := htmlRedirectCommand(t, writeImportFixture(t, ext))
			if err := runImportCommand(cmd, nil, docImportFlowConfig()); err != nil {
				t.Fatalf("runImportCommand(%s) error = %v, want upload redirect success", ext, err)
			}
			if caller.tool != "commit_uploaded_file" {
				t.Fatalf("%s last tool = %q, want commit_uploaded_file", ext, caller.tool)
			}
		}
	})

	t.Run("extensionless file is redirected with a readable label", func(t *testing.T) {
		caller := &scriptedToolCaller{steps: uploadSteps}
		installScriptedCaller(t, caller)
		var warnings bytes.Buffer
		deps.Out.errW = &warnings
		SetHTTPPutFile(func(context.Context, string, map[string]string, string, int64) error { return nil })
		t.Cleanup(func() { SetHTTPPutFile(nil) })

		noExt := filepath.Join(t.TempDir(), "README")
		if err := os.WriteFile(noExt, []byte("plain"), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := htmlRedirectCommand(t, noExt)
		if err := runImportCommand(cmd, nil, docImportFlowConfig()); err != nil {
			t.Fatalf("runImportCommand() error = %v, want upload redirect success", err)
		}
		if !strings.Contains(warnings.String(), "无扩展名") {
			t.Fatalf("warning must label the extensionless file, got %q", warnings.String())
		}
	})

	t.Run("importable formats never redirect", func(t *testing.T) {
		for _, ext := range []string{"docx", "md", "xlsx"} {
			cfg := docImportFlowConfig()
			cfg.uploadRedirect = func(*cobra.Command, []string) error {
				t.Fatalf("importable format %s must not redirect", ext)
				return nil
			}
			cmd := htmlRedirectCommand(t, writeImportFixture(t, ext))
			if got := importUploadRedirect(cmd, nil, cfg); got != nil {
				t.Fatalf("importUploadRedirect(%s) must return nil", ext)
			}
		}
	})

	t.Run("missing file falls through to the import required-flag error", func(t *testing.T) {
		installScriptedCaller(t, &scriptedToolCaller{})
		cmd := htmlRedirectCommand(t, "")
		err := runImportCommand(cmd, nil, docImportFlowConfig())
		if err == nil || !strings.Contains(err.Error(), "--file is required") {
			t.Fatalf("runImportCommand() error = %v, want --file required", err)
		}
	})

	t.Run("sheet import keeps rejecting html without redirect", func(t *testing.T) {
		installScriptedCaller(t, &scriptedToolCaller{})
		cmd := htmlRedirectCommand(t, writeImportFixture(t, "html"))
		if err := cmd.Flags().Set("workspace", "ws-1"); err != nil {
			t.Fatal(err)
		}
		err := runImportCommand(cmd, nil, sheetImportFlowConfig())
		if err == nil || !strings.Contains(err.Error(), "unsupported file format") {
			t.Fatalf("sheet import error = %v, want unsupported file format", err)
		}
	})
}
