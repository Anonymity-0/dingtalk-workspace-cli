package helpers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/runtimeannotate"
)

func TestCrossPlatformCoverageSetDropdownSourceRangeCommand(t *testing.T) {
	caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{"success":true}`}}}
	installScriptedCaller(t, caller)
	err := executeDimensionCoverage(t, "set-dropdown",
		"--node", "node-1",
		"--sheet-id", "target-sheet",
		"--range", "B2:B100",
		"--source-sheet-id", "source-sheet",
		"--source-range", "$t$1:$t$3",
		"--multi-select",
	)
	if err != nil {
		t.Fatalf("set SourceRange dropdown: %v", err)
	}
	if caller.tool != "set_dropdown_lists" {
		t.Fatalf("tool = %q, want set_dropdown_lists", caller.tool)
	}
	if _, exists := caller.args["options"]; exists {
		t.Fatalf("SourceRange request must omit options: %#v", caller.args)
	}
	source, ok := caller.args["sourceRange"].(map[string]any)
	if !ok {
		t.Fatalf("sourceRange = %#v", caller.args["sourceRange"])
	}
	if source["sheetId"] != "source-sheet" || source["a1Notation"] != "$t$1:$t$3" {
		t.Fatalf("sourceRange = %#v", source)
	}
	if caller.args["enableMultiSelect"] != true {
		t.Fatalf("enableMultiSelect = %#v", caller.args["enableMultiSelect"])
	}
}

func TestCrossPlatformCoverageSetDropdownInlineCommandStillOmitsSourceRange(t *testing.T) {
	caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{"success":true}`}}}
	installScriptedCaller(t, caller)
	err := executeDimensionCoverage(t, "set-dropdown",
		"--node", "node-1",
		"--sheet-id", "sheet-1",
		"--range", "A1:A3",
		"--options", `[{"value":"one","color":"#ff0000"}]`,
	)
	if err != nil {
		t.Fatalf("set inline dropdown: %v", err)
	}
	if _, exists := caller.args["sourceRange"]; exists {
		t.Fatalf("inline request must omit sourceRange: %#v", caller.args)
	}
	options, ok := caller.args["options"].([]map[string]any)
	if !ok || len(options) != 1 || options[0]["value"] != "one" {
		t.Fatalf("options = %#v", caller.args["options"])
	}
}

func TestCrossPlatformCoverageSetDropdownModeConstraints(t *testing.T) {
	base := []string{"--node", "node", "--sheet-id", "sheet", "--range", "A1"}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "neither", want: "one of the flags"},
		{name: "both", args: []string{"--options", `[{"value":"one"}]`, "--source-sheet-id", "source", "--source-range", "A1:A3"}, want: "none of the others"},
		{name: "source sheet without range", args: []string{"--options", `[{"value":"one"}]`, "--source-sheet-id", "source"}, want: "must all be set"},
		{name: "range without source sheet", args: []string{"--source-range", "A1:A3"}, want: "must all be set"},
		{name: "sheet prefix", args: []string{"--source-sheet-id", "source", "--source-range", "Sheet2!A1:A3"}, want: "不能包含工作表前缀"},
		{name: "formula", args: []string{"--source-sheet-id", "source", "--source-range", "=A1:A3"}, want: "必须是单一连续区域"},
		{name: "multi region", args: []string{"--source-sheet-id", "source", "--source-range", "A1:A3,C1:C3"}, want: "必须是单一连续区域"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			installScriptedCaller(t, caller)
			args := append(append([]string{}, base...), tc.args...)
			err := executeDimensionCoverage(t, "set-dropdown", args...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want contains %q", err, tc.want)
			}
			if caller.calls != 0 {
				t.Fatalf("calls = %d, validation must fail before request", caller.calls)
			}
		})
	}
}

func TestCrossPlatformCoverageSetDropdownSchemaConstraints(t *testing.T) {
	cmd := dimensionCoverageCommand(t, "set-dropdown")
	constraints := runtimeannotate.CommandConstraints(cmd)
	want := runtimeannotate.RuntimeSchemaConstraints{
		MutuallyExclusive: [][]string{{"options", "source-range"}},
		RequireOneOf:      [][]string{{"options", "source-range"}},
		RequireTogether:   [][]string{{"source-sheet-id", "source-range"}},
	}
	gotJSON, _ := json.Marshal(constraints)
	wantJSON, _ := json.Marshal(runtimeannotate.NormalizeConstraints(want))
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("constraints = %s, want %s", gotJSON, wantJSON)
	}
}

func TestCrossPlatformCoverageBatchSetDropdownSourceRange(t *testing.T) {
	got, err := translateBatchOp(map[string]any{
		"toolName": "set-dropdown",
		"input": map[string]any{
			"sheet-id":        "target-sheet",
			"range":           "B2:B100",
			"source-sheet-id": "source-sheet",
			"source-range":    "T:T",
			"multi-select":    true,
		},
	})
	if err != nil {
		t.Fatalf("translate SourceRange batch op: %v", err)
	}
	input := got["input"].(map[string]any)
	if _, exists := input["options"]; exists {
		t.Fatalf("SourceRange batch input must omit options: %#v", input)
	}
	source := input["sourceRange"].(map[string]any)
	if source["sheetId"] != "source-sheet" || source["a1Notation"] != "T:T" {
		t.Fatalf("sourceRange = %#v", source)
	}

	invalid := []map[string]any{
		{},
		{"options": []any{map[string]any{"value": "one"}}, "source-sheet-id": "source", "source-range": "A1:A3"},
		{"source-range": "A1:A3"},
		{"source-sheet-id": "source", "source-range": "A1:A3", "colors": []any{"#fff"}},
		{"source-sheet-id": "source", "source-range": "Sheet2!A1:A3"},
	}
	for _, value := range invalid {
		if _, err := translateBatchOp(map[string]any{"toolName": "set-dropdown", "input": value}); err == nil {
			t.Errorf("invalid batch SourceRange input %#v returned nil", value)
		}
	}
}

func TestCrossPlatformCoverageValidateSourceRangeDataValidation(t *testing.T) {
	valid := []any{
		map[string]any{"type": "dropdown", "sourceRange": map[string]any{"sheetId": "source", "a1Notation": "T1:T3"}},
		map[string]any{"type": "dropdown", "sourceRange": map[string]any{"sheetId": "missing-sheet", "a1Notation": "not-validated-locally"}, "enableMultiSelect": true},
	}
	for _, value := range valid {
		if err := validateDataValidation(value, "dv"); err != nil {
			t.Errorf("valid SourceRange data validation %#v: %v", value, err)
		}
	}

	invalid := []any{
		map[string]any{"type": "dropdown", "options": []any{map[string]any{"value": "one"}}, "sourceRange": map[string]any{"sheetId": "source", "a1Notation": "T1:T3"}},
		map[string]any{"type": "dropdown", "sourceRange": "T1:T3"},
		map[string]any{"type": "dropdown", "sourceRange": map[string]any{"a1Notation": "T1:T3"}},
		map[string]any{"type": "dropdown", "sourceRange": map[string]any{"sheetId": "source"}},
		map[string]any{"type": "dropdown", "sourceRange": map[string]any{"sheetId": "source", "a1Notation": "T1:T3", "colors": []any{"#fff"}}},
		map[string]any{"type": "dropdown", "sourceRange": map[string]any{"sheetId": "source", "a1Notation": "T1:T3"}, "enableMultiSelect": "yes"},
	}
	for _, value := range invalid {
		if err := validateDataValidation(value, "dv"); err == nil {
			t.Errorf("invalid SourceRange data validation %#v returned nil", value)
		}
	}
}
