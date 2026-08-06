// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package aitable

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/spf13/cobra"
)

func runAITableCompositeCLI(t *testing.T, caller *upsertByKeyCaller, command string, args ...string) (string, error) {
	t.Helper()
	helpers.InitDepsForTest(t, caller)
	root := &cobra.Command{Use: "dws", SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().Bool("yes", false, "")
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().String("format", "json", "")
	root.AddCommand(shortcut.Commands()...)
	stdout := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"aitable", command}, args...))
	err := root.Execute()
	return stdout.String(), err
}

func TestCrossPlatformCoverageBaseSchemaSnapshotE2E(t *testing.T) {
	t.Run("one table with legal empty fields and views", func(t *testing.T) {
		caller := &upsertByKeyCaller{steps: []upsertByKeyStep{
			{text: `{"data":{"baseId":"base","tables":[{"tableId":"t1","name":"任务"}]}}`},
			{text: `{"data":{"tables":[{"tableId":"t1","name":"任务"}]}}`},
			{text: `{"data":{"fields":[]}}`},
			{text: `{"data":{"views":[]}}`},
		}}
		out, err := runAITableCompositeCLI(t, caller, "+base-schema-snapshot", "--base-id", "base")
		if err != nil {
			t.Fatalf("snapshot error = %v", err)
		}
		for _, want := range []string{`"status": "success"`, `"tableCount": 1`, `"fields": []`, `"views": []`} {
			if !strings.Contains(out, want) {
				t.Fatalf("snapshot output missing %s: %s", want, out)
			}
		}
	})

	t.Run("empty tables is explicit success", func(t *testing.T) {
		caller := &upsertByKeyCaller{steps: []upsertByKeyStep{{text: `{"baseId":"base","tables":[]}`}}}
		out, err := runAITableCompositeCLI(t, caller, "+base-schema-snapshot", "--base-id", "base")
		if err != nil || !strings.Contains(out, `"tableCount": 0`) || len(caller.calls) != 1 {
			t.Fatalf("empty snapshot = output:%q err:%v calls:%#v", out, err, caller.calls)
		}
	})

	t.Run("missing tables contract fails", func(t *testing.T) {
		caller := &upsertByKeyCaller{steps: []upsertByKeyStep{{text: `{"baseId":"base","data":{}}`}}}
		out, err := runAITableCompositeCLI(t, caller, "+base-schema-snapshot", "--base-id", "base")
		if err == nil || out != "" {
			t.Fatalf("missing tables = output:%q err:%v", out, err)
		}
	})
}

func bootstrapFields(count int) []any {
	fields := make([]any, 0, count)
	for index := 0; index < count; index++ {
		fields = append(fields, map[string]any{"fieldName": fmt.Sprintf("F%02d", index), "type": "text"})
	}
	return fields
}

func marshalBootstrapTables(t *testing.T, fields []any) string {
	t.Helper()
	if fields == nil {
		fields = []any{}
	}
	raw, err := json.Marshal([]any{map[string]any{"name": "任务", "fields": fields}})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func fieldReadBackJSON(t *testing.T, fields []any) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestCrossPlatformCoverageBaseBootstrapCreatesChunksAndVerifiesE2E(t *testing.T) {
	fields := bootstrapFields(16)
	caller := &upsertByKeyCaller{steps: []upsertByKeyStep{
		{text: `{"data":{"baseId":"b-new"}}`},
		{text: `{"baseId":"b-new","tables":[]}`},
		{text: `{"data":{"tableId":"t-new"}}`},
		{text: `{"createdFields":[{"fieldId":"f16"}]}`},
		{text: `{"tables":[{"tableId":"t-new","name":"任务"}]}`},
		{text: fieldReadBackJSON(t, fields)},
	}}
	out, err := runAITableCompositeCLI(t, caller, "+base-bootstrap", "--name", "项目", "--tables", marshalBootstrapTables(t, fields))
	if err != nil {
		t.Fatalf("bootstrap error = %v", err)
	}
	for _, want := range []string{`"baseId": "b-new"`, `"tableId": "t-new"`, `"status": "verified"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("bootstrap output missing %s: %s", want, out)
		}
	}
	if len(caller.calls) != 6 || caller.calls[2].tool != "create_table" || caller.calls[3].tool != "create_fields" {
		t.Fatalf("bootstrap calls = %#v", caller.calls)
	}
	if got := len(caller.calls[2].args["fields"].([]any)); got != 15 {
		t.Fatalf("create_table fields = %d, want 15", got)
	}
	if got := len(caller.calls[3].args["fields"].([]any)); got != 1 {
		t.Fatalf("create_fields fields = %d, want 1", got)
	}
}

func TestCrossPlatformCoverageBaseBootstrapUnknownAndDryRunE2E(t *testing.T) {
	t.Run("empty create response is unknown and not retry-safe", func(t *testing.T) {
		caller := &upsertByKeyCaller{steps: []upsertByKeyStep{{text: ""}}}
		out, err := runAITableCompositeCLI(t, caller, "+base-bootstrap", "--name", "项目", "--tables", marshalBootstrapTables(t, nil))
		if err == nil || out != "" {
			t.Fatalf("unknown base create = output:%q err:%v", out, err)
		}
		var typed *apperrors.Error
		if !errors.As(err, &typed) || typed.Reason != "aitable_composite_unknown" || typed.Retryable {
			t.Fatalf("unknown base error = %#v", err)
		}
	})

	t.Run("dry run validates and makes no calls", func(t *testing.T) {
		caller := &upsertByKeyCaller{dryRun: true}
		out, err := runAITableCompositeCLI(t, caller, "+base-bootstrap", "--name", "项目", "--tables", marshalBootstrapTables(t, nil), "--dry-run")
		if err != nil || len(caller.calls) != 0 || !strings.Contains(out, `"status": "planned"`) {
			t.Fatalf("bootstrap dry run = output:%q err:%v calls:%#v", out, err, caller.calls)
		}
	})
}
