package app

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCrossPlatformCoverageOATemplateDeliveredContract(t *testing.T) {
	for _, tc := range []struct{ command, tool string }{{"list", "list_manage_templates"}, {"detail", "get_template_detail"}} {
		t.Run(tc.command, func(t *testing.T) {
			root := NewRootCommand()
			path := "oa approval template " + tc.command
			command := exactCommandForTest(root, path)
			if command == nil {
				t.Fatal("missing executable")
			}
			var buf bytes.Buffer
			root.SetOut(&buf)
			root.SetArgs([]string{"schema", path, "--format", "json"})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			var leaf map[string]any
			if err := json.Unmarshal(buf.Bytes(), &leaf); err != nil {
				t.Fatal(err)
			}
			if leaf["canonical_path"] != "oa."+tc.tool || leaf["effect"] != "read" || leaf["confirmation"] != "not_required" {
				t.Fatalf("identity/safety: %#v", leaf)
			}
			if schemaInterfaceObject(leaf["interface_ref"])["rpc_name"] != tc.tool {
				t.Fatal("wrong MCP interface")
			}
			if leaf["primary_cli_path"] != path {
				t.Fatalf("primary path = %v, want %s", leaf["primary_cli_path"], path)
			}
			if leaf["result"] == nil {
				t.Fatal("missing result contract")
			}
			params := schemaContractMap(leaf["parameters"])
			if tc.command == "list" {
				if len(params) != 0 {
					t.Fatalf("unexpected parameters: %#v", params)
				}
			} else {
				if command.Flags().Lookup("process-code") == nil || command.Flags().Lookup("template-code") != nil {
					t.Fatal("template detail help flags do not match the new contract")
				}
				p := params["process-code"]
				if len(params) != 1 || p["type"] != "string" || p["property"] != "processCodes" || p["interface_type"] != "array" || p["required"] != true {
					t.Fatalf("parameter contract: %#v", params)
				}
			}
		})
	}
}
