package helpers

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
)

func TestCalendarRecurrenceParamDeclsCoverHintPins(t *testing.T) {
	decls := calendarRecurrenceParamDecls()
	byName := map[string]corecmd.ParamDecl{}
	for _, d := range decls {
		byName[d.Name] = d
	}
	want := map[string]string{
		"recurrence-day-of-month": "recurrence-type is absoluteMonthly or absoluteYearly",
		"recurrence-days-of-week": "recurrence-type is weekly or relativeMonthly",
		"recurrence-index":        "recurrence-type is relativeMonthly",
		"recurrence-interval":     "any recurrence-* flag is provided",
		"recurrence-type":         "any recurrence-* flag is provided",
		"recurrence-end-date":     "recurrence-range-type is endDate",
		"recurrence-count":        "recurrence-range-type is numbered",
		"recurrence-range-type":   "any recurrence-* flag is provided",
	}
	if len(decls) != len(want) {
		t.Fatalf("calendarRecurrenceParamDecls len = %d, want %d: %#v", len(decls), len(want), decls)
	}
	for name, when := range want {
		d, ok := byName[name]
		if !ok {
			t.Fatalf("missing ParamDecl %q", name)
		}
		if d.Required == nil || *d.Required {
			t.Fatalf("%s Required = %#v, want explicit false", name, d.Required)
		}
		if d.RequiredWhen != when {
			t.Fatalf("%s RequiredWhen = %q, want %q", name, d.RequiredWhen, when)
		}
	}
}
