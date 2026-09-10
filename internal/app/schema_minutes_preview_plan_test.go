// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package app

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestCrossPlatformCoverageMinutesPreviewPlanFinalDelivery(t *testing.T) {
	for _, tc := range []struct {
		path   string
		fields []string
		cue    string
	}{
		{"minutes +share", []string{"permission", "options", "failurePolicy"}, "预览显示目标和失败策略"},
		{"minutes +unshare", []string{"failurePolicy"}, "预览显示目标和失败策略"},
		{"minutes +upload", []string{"options", "completeTimeoutSeconds", "pollIntervalSeconds"}, "预览包含显式语言"},
		{"minutes +upload-and-notify", []string{"options", "completeTimeoutSeconds", "pollIntervalSeconds"}, "预览包含显式语言"},
	} {
		full := executeShortcutSchemaQuery(t, "--cli-path", tc.path)
		compact := executeShortcutSchemaQuery(t, "--cli-path", tc.path, "--compact")
		if !reflect.DeepEqual(full["result"], compact["result"]) {
			t.Fatalf("%s result drift", tc.path)
		}
		properties := schemaContractMap(schemaContractMap(schemaContractMap(full["result"])["data_schema"])["properties"])
		for _, field := range tc.fields {
			if schemaContractString(schemaContractMap(properties[field])["description"]) == "" {
				t.Fatalf("%s missing %s", tc.path, field)
			}
		}
		if full["confirmation"] != "user_required" {
			t.Fatalf("%s confirmation drift", tc.path)
		}
		if !strings.Contains(schemaContractString(full["description"]), tc.cue) {
			t.Fatalf("%s Schema lost intent", tc.path)
		}
		root := NewRootCommand()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetErr(&output)
		root.SetArgs(append(strings.Fields(tc.path), "--help"))
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), tc.cue) {
			t.Fatalf("%s Help lost intent", tc.path)
		}
	}
}
