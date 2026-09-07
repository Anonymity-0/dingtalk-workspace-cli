package helpers

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func runPsqlCLI(t *testing.T, caller *recordQueryE2ECaller, args ...string) (string, error) {
	t.Helper()
	testseam.Protect(t, &deps)
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	InitDeps(caller)
	out := &bytes.Buffer{}
	cmd := newAitablePsqlCommand()
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs(args)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	returnValue := cmd.Execute()
	return out.String(), returnValue
}

func TestAitablePsqlListTables(t *testing.T) {
	caller := &recordQueryE2ECaller{steps: []recordQueryE2EStep{{result: textToolResult(
		`{"status":"success","data":[{"tableId":"tbl1","tableName":"项目表","description":"项目进度管理"}]}`)}}}
	out, err := runPsqlCLI(t, caller, "-d", "base1", "-l")
	if err != nil {
		t.Fatalf("psql list failed: %v", err)
	}
	if !strings.Contains(out, "项目表") || !strings.Contains(out, "tbl1") ||
		!strings.Contains(out, "Description") || !strings.Contains(out, "项目进度管理") {
		t.Fatalf("unexpected output: %s", out)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != "otable_pg_list_tables" {
		t.Fatalf("unexpected calls: %#v", caller.calls)
	}
}

func TestAitablePsqlDescribeAllProperties(t *testing.T) {
	caller := &recordQueryE2ECaller{steps: []recordQueryE2EStep{{result: textToolResult(
		`{"status":"success","data":{"tableId":"tbl1","tableName":"项目表","description":"项目进度管理","columns":[{"columnName":"人员字段1_id","pgType":"text[]","fieldId":"fld1","attribute":"id","description":"项目负责人"}]}}`)}}}
	out, err := runPsqlCLI(t, caller, "-d", "base1", "-t", "tbl1", "--all-properties")
	if err != nil {
		t.Fatalf("psql describe failed: %v", err)
	}
	if !strings.Contains(out, "人员字段1_id") || !strings.Contains(out, "项目进度管理") ||
		!strings.Contains(out, "项目负责人") || caller.calls[0].args["allProperties"] != true {
		t.Fatalf("unexpected output/call: %s %#v", out, caller.calls)
	}
}

func TestAitablePsqlExecuteExpanded(t *testing.T) {
	caller := &recordQueryE2ECaller{steps: []recordQueryE2EStep{{result: textToolResult(
		`{"status":"success","data":{"columns":[{"columnName":"人员字段1_id","pgType":"text[]"}],"rows":[[["user_001","user_002"]]],"rowCount":1,"truncated":false}}`)}}}
	out, err := runPsqlCLI(t, caller, "-d", "base1", "-x", "-c", "SELECT 人员字段1_id FROM 项目表")
	if err != nil {
		t.Fatalf("psql execute failed: %v", err)
	}
	if !strings.Contains(out, "-[ RECORD 1 ]-") || !strings.Contains(out, `["user_001","user_002"]`) {
		t.Fatalf("unexpected output: %s", out)
	}
	if caller.calls[0].tool != "otable_pg_execute" || caller.calls[0].args["limit"] != 200 {
		t.Fatalf("unexpected call: %#v", caller.calls[0])
	}
	if _, exists := caller.calls[0].args["tableId"]; exists {
		t.Fatalf("execute should not send tableId: %#v", caller.calls[0])
	}
}

func TestAitablePsqlRejectsAmbiguousMode(t *testing.T) {
	out, err := runPsqlCLI(t, &recordQueryE2ECaller{}, "-d", "base1", "-l", "-t", "tbl1")
	if err == nil || !strings.Contains(err.Error(), "exactly one mode") {
		t.Fatalf("error = %v, output = %s", err, out)
	}
}
