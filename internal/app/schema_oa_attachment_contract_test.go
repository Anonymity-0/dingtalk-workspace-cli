// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCrossPlatformCoverageOAAttachmentDeliveredSchemaMatchesExecutableHelp(t *testing.T) {
	tests := []struct {
		cliPath     string
		canonical   string
		rpc         string
		description string
		effect      string
	}{
		{
			cliPath:     "oa approval attachment download-url",
			canonical:   "oa.get_attachment_download_url",
			rpc:         "get_attachment_download_url",
			description: "获取审批附件下载授权并生成临时下载链接",
			effect:      "read",
		},
		{
			cliPath:     "oa approval attachment authorize-download",
			canonical:   "oa.auth_download_file",
			rpc:         "auth_download_file",
			description: "批量授权当前用户下载指定的审批钉盘文件",
			effect:      "write",
		},
		{
			cliPath:     "oa approval attachment authorize-preview",
			canonical:   "oa.auth_preview_attachment",
			rpc:         "auth_preview_attachment",
			description: "批量授权当前用户预览审批单中的附件",
			effect:      "write",
		},
	}

	root := NewRootCommand()
	for _, test := range tests {
		t.Run(test.rpc, func(t *testing.T) {
			command := exactCommandForTest(root, test.cliPath)
			if command == nil {
				t.Fatalf("executable command %q is missing", test.cliPath)
			}

			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs([]string{"schema", test.cliPath, "--format", "json"})
			if err := root.Execute(); err != nil {
				t.Fatalf("execute delivery schema leaf: %v; stderr=%s", err, stderr.String())
			}

			var tool map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &tool); err != nil {
				t.Fatalf("decode delivery schema leaf: %v", err)
			}
			if got := schemaContractString(tool["canonical_path"]); got != test.canonical {
				t.Fatalf("canonical_path = %q, want %q", got, test.canonical)
			}
			if got := schemaContractString(tool["primary_cli_path"]); got != test.cliPath {
				t.Fatalf("primary_cli_path = %q, want %q", got, test.cliPath)
			}
			if got := schemaContractString(tool["description"]); !strings.HasPrefix(got, test.description) {
				t.Fatalf("description = %q, want prefix %q", got, test.description)
			}
			if got := schemaContractString(tool["interface_mode"]); got != "mcp" {
				t.Fatalf("interface_mode = %q, want mcp", got)
			}
			if got := schemaContractString(tool["availability"]); got != "available" {
				t.Fatalf("availability = %q, want available", got)
			}
			interfaceRef := schemaInterfaceObject(tool["interface_ref"])
			if got := schemaContractString(interfaceRef["product_id"]); got != "oa" {
				t.Fatalf("interface_ref.product_id = %q, want oa", got)
			}
			if got := schemaContractString(interfaceRef["rpc_name"]); got != test.rpc {
				t.Fatalf("interface_ref.rpc_name = %q, want %q", got, test.rpc)
			}
			if got := schemaContractString(tool["effect"]); got != test.effect {
				t.Fatalf("effect = %q, want %q", got, test.effect)
			}
			if got := schemaContractString(tool["risk"]); got != "low" {
				t.Fatalf("risk = %q, want low", got)
			}
			if got := schemaContractString(tool["confirmation"]); got != "not_required" {
				t.Fatalf("confirmation = %q, want not_required", got)
			}
			if got := schemaContractString(tool["idempotency"]); got != "idempotent" {
				t.Fatalf("idempotency = %q, want idempotent", got)
			}
			if problem := schemaHelpFlagCompletenessProblem(test.canonical, test.cliPath, command, tool); problem != "" {
				t.Fatal(problem)
			}
		})
	}
}
