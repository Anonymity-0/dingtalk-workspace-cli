// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package minutes

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCrossPlatformCoverageMinutesPermissionPlanScope(t *testing.T) {
	for _, cover := range []string{"", "--cover", "--cover=false"} {
		args := []string{"minutes", "+share", "--ids", "u1,u2", "--member-uids", "m1,m2", "--permission", "view", "--sub-resources", "Summary", "--failure-policy", "continue"}
		if cover != "" {
			args = append(args, cover)
		}
		previewCaller := &minutesE2ECaller{}
		p, _, err := runMinutesAlignmentCLI(t, previewCaller, append(append([]string{}, args...), "--dry-run")...)
		if err != nil || len(previewCaller.counts) != 0 || p["executed"] != false || p["permission"] != "view" || p["failurePolicy"] != "continue" {
			t.Fatalf("plan=%#v err=%v", p, err)
		}
		options := p["options"].(map[string]any)
		if options["policyId"] != float64(4) || !reflect.DeepEqual(options["roleSubResourceIds"], []any{"Summary"}) {
			t.Fatalf("options=%#v", options)
		}
		expectedCover := map[string]any{"--cover": "true", "--cover=false": "false"}[cover]
		if options["coverPermission"] != expectedCover {
			t.Fatalf("cover=%#v want=%#v", options["coverPermission"], expectedCover)
		}
		live := &minutesE2ECaller{}
		if _, _, err := runMinutesAlignmentCLI(t, live, append(append([]string{}, args...), "--yes")...); err != nil {
			t.Fatal(err)
		}
		calls := live.arguments["minutes/add_member_permission"]
		if len(calls) != 2 {
			t.Fatalf("calls=%v", calls)
		}
		for _, call := range calls {
			for key, expected := range options {
				raw, err := json.Marshal(call[key])
				if err != nil {
					t.Fatal(err)
				}
				var actual any
				if err := json.Unmarshal(raw, &actual); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, expected) {
					t.Fatalf("%s actual=%#v plan=%#v", key, actual, expected)
				}
			}
		}
	}
	c := &minutesE2ECaller{}
	p, _, err := runMinutesAlignmentCLI(t, c, "minutes", "+unshare", "--ids", "u1,u2", "--member-uids", "m1,m2", "--failure-policy", "continue", "--dry-run")
	if err != nil || len(c.counts) != 0 || p["failurePolicy"] != "continue" || p["options"] != nil || p["permission"] != nil || p["memberCount"] != float64(2) {
		t.Fatalf("unshare=%#v err=%v", p, err)
	}
	c = &minutesE2ECaller{}
	p, _, err = runMinutesAlignmentCLI(t, c, "minutes", "+share", "--id", "u1", "--member-uids", "m1", "--permission", "view", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	options := p["options"].(map[string]any)
	if options["coverPermission"] != nil || options["roleSubResourceIds"] != nil || len(c.counts) != 0 {
		t.Fatal("invented unset options or remote call")
	}
}
