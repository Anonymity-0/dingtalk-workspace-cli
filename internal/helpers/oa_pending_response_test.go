package helpers

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func TestOAPendingResponseEnvelope(t *testing.T) {
	for _, format := range []string{"json", "raw"} {
		for _, tc := range []struct {
			name, response string
			wantError      bool
		}{
			{"string success", `{"success":"true","error_code":"0","result":{"id":9007199254740993,"success":"false","error_code":"007"}}`, false},
			{"typed success", `{"success":true,"errorCode":0,"result":{"id":9007199254740993,"success":"false","error_code":"007"}}`, false},
			{"string failure", `{"success":"false","error_code":"0"}`, true},
			{"typed failure", `{"success":false,"errorCode":0}`, true},
			{"nonzero code", `{"success":"true","error_code":"123"}`, true},
			{"invalid success", `{"success":"unknown","error_code":"0"}`, true},
		} {
			t.Run(format+"/"+tc.name, func(t *testing.T) {
				caller := &scriptedToolCaller{format: format, steps: []scriptedToolStep{{text: tc.response}}}
				installScriptedCaller(t, caller)
				testseam.Swap(t, &os.Args, []string{"dws", "oa"})
				var out bytes.Buffer
				deps.Out.w = &out
				cmd := newOaCommand()
				cmd.SilenceErrors, cmd.SilenceUsage = true, true
				cmd.SetArgs([]string{"approval", "list-pending", "--page", "1", "--limit", "20"})
				err := cmd.Execute()
				if tc.wantError {
					if err == nil || out.Len() != 0 {
						t.Fatalf("want error without success output, got err=%v output=%s", err, &out)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				var body map[string]json.RawMessage
				if err := json.Unmarshal(out.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if string(body["success"]) != "true" || string(body["errorCode"]) != "0" || body["error_code"] != nil {
					t.Fatalf("unexpected envelope: %s", &out)
				}
				var result map[string]json.RawMessage
				if err := json.Unmarshal(body["result"], &result); err != nil {
					t.Fatal(err)
				}
				if string(result["id"]) != "9007199254740993" || string(result["success"]) != `"false"` || string(result["error_code"]) != `"007"` {
					t.Fatalf("business data changed: %s", body["result"])
				}
				if caller.server != "oa" || caller.tool != "get_todo_tasks" {
					t.Fatalf("unexpected interface: %s/%s", caller.server, caller.tool)
				}
				if caller.calls != 1 {
					t.Fatalf("calls = %d", caller.calls)
				}
			})
		}
	}
}
