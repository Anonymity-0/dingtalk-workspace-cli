package app

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCrossPlatformCoverageOATemplateDeliveredContract(t *testing.T) {
	for _, tc := range []struct{ command, tool string }{{"list-manage-templates", "list_manage_templates"}, {"get-template-detail", "get_template_detail"}} {
		t.Run(tc.command, func(t *testing.T) {
			root := NewRootCommand()
			path := "oa approval " + tc.command
			if exactCommandForTest(root, path) == nil {
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
			if leaf["result"] == nil {
				t.Fatal("missing result contract")
			}
			params := schemaContractMap(leaf["parameters"])
			if tc.command == "list-manage-templates" {
				if len(params) != 0 {
					t.Fatalf("unexpected parameters: %#v", params)
				}
			} else {
				p := params["process-code"]
				if len(params) != 1 || p["type"] != "string" || p["property"] != "processCodes" || p["interface_type"] != "array" || p["required"] != true {
					t.Fatalf("parameter contract: %#v", params)
				}
			}
		})
	}
}
