// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package minutes

import "testing"

func TestCrossPlatformCoverageMinutesListPreviewAndPageAllE2E(t *testing.T) {
	previewCaller := &minutesE2ECaller{responses: map[string][]string{
		"minutes/list_by_keyword_and_time_range": {`{"success":true,"result":{"itemList":[{"taskUuid":"u1","title":"第一条"}],"hasNext":true,"nextToken":"n2"}}`},
	}}
	preview, _, err := runMinutesAlignmentCLI(t, previewCaller, "minutes", "+list-mine", "--limit", "1")
	if err != nil || preview["complete"] != false || preview["endpointExhausted"] != false || preview["nextToken"] != "n2" || preview["pages"] != float64(1) {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}

	allCaller := &minutesE2ECaller{responses: map[string][]string{
		"minutes/list_by_keyword_and_time_range": {
			`{"success":true,"result":{"itemList":[{"taskUuid":"u1","title":"第一条"}],"hasNext":true,"nextToken":"n2"}}`,
			`{"success":true,"result":{"itemList":[{"taskUuid":"u1","title":"第一条"},{"taskUuid":"u2","title":"第二条"}],"hasNext":false}}`,
		},
	}}
	all, _, err := runMinutesAlignmentCLI(t, allCaller, "minutes", "+list-mine", "--limit", "1", "--page-all")
	if err != nil || all["complete"] != true || all["endpointExhausted"] != true || all["count"] != float64(2) || all["pages"] != float64(2) {
		t.Fatalf("page-all=%#v err=%v", all, err)
	}
	if calls := allCaller.arguments["minutes/list_by_keyword_and_time_range"]; len(calls) != 2 || calls[0]["belongingConditionId"] != "created" || calls[1]["nextToken"] != "n2" {
		t.Fatalf("page-all calls=%#v", calls)
	}
}

func TestCrossPlatformCoverageMinutesAccessibleListMergesMineAndSharedE2E(t *testing.T) {
	caller := &minutesE2ECaller{responses: map[string][]string{
		"minutes/list_by_keyword_and_time_range": {
			`{"success":true,"result":{"itemList":[{"taskUuid":"u1","title":"自有"}],"hasNext":false}}`,
			`{"success":true,"result":{"itemList":[{"taskUuid":"u1","title":"重复"},{"taskUuid":"u2","title":"共享"}],"hasNext":false}}`,
		},
	}}
	payload, _, err := runMinutesAlignmentCLI(t, caller, "minutes", "+list-all", "--page-all")
	if err != nil || payload["complete"] != true || payload["count"] != float64(2) || payload["pages"] != float64(2) {
		t.Fatalf("accessible=%#v err=%v", payload, err)
	}
	ledger, ok := payload["scopeLedger"].([]any)
	if !ok || len(ledger) != 2 {
		t.Fatalf("scope ledger=%#v", payload["scopeLedger"])
	}
	calls := caller.arguments["minutes/list_by_keyword_and_time_range"]
	if len(calls) != 2 || calls[0]["belongingConditionId"] != "created" || calls[1]["belongingConditionId"] != "shared" {
		t.Fatalf("accessible calls=%#v", calls)
	}
}

func TestCrossPlatformCoverageMinutesAccessiblePreviewNeverClaimsUnionCompleteE2E(t *testing.T) {
	caller := &minutesE2ECaller{responses: map[string][]string{
		"minutes/list_by_keyword_and_time_range": {`{"success":true,"result":{"itemList":[],"hasNext":false}}`},
	}}
	payload, _, err := runMinutesAlignmentCLI(t, caller, "minutes", "+list-all")
	if err != nil || payload["complete"] != false || payload["endpointExhausted"] != true || payload["nextAction"] == "" {
		t.Fatalf("accessible preview=%#v err=%v", payload, err)
	}
	calls := caller.arguments["minutes/list_by_keyword_and_time_range"]
	if len(calls) != 1 || calls[0]["belongingConditionId"] != "noLimit" {
		t.Fatalf("preview calls=%#v", calls)
	}
}

func TestCrossPlatformCoverageMinutesListPaginationFailsClosedE2E(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		caller := &minutesE2ECaller{responses: map[string][]string{
			"minutes/list_by_keyword_and_time_range": {`{"success":true,"result":{"itemList":[],"hasNext":true}}`},
		}}
		if _, _, err := runMinutesAlignmentCLI(t, caller, "minutes", "+list-mine", "--page-all"); err == nil {
			t.Fatal("missing nextToken accepted")
		}
	})

	t.Run("cursor cycle", func(t *testing.T) {
		caller := &minutesE2ECaller{responses: map[string][]string{
			"minutes/list_by_keyword_and_time_range": {
				`{"success":true,"result":{"itemList":[{"taskUuid":"u1"}],"hasNext":true,"nextToken":"same"}}`,
				`{"success":true,"result":{"itemList":[{"taskUuid":"u2"}],"hasNext":true,"nextToken":"same"}}`,
			},
		}}
		payload, _, err := runMinutesAlignmentCLI(t, caller, "minutes", "+list-mine", "--page-all")
		if err == nil || payload["complete"] != false || payload["nextToken"] != "same" {
			t.Fatalf("cycle payload=%#v err=%v", payload, err)
		}
	})

	t.Run("page limit", func(t *testing.T) {
		caller := &minutesE2ECaller{responses: map[string][]string{
			"minutes/list_by_keyword_and_time_range": {`{"success":true,"result":{"itemList":[{"taskUuid":"u1"}],"hasNext":true,"nextToken":"n2"}}`},
		}}
		payload, _, err := runMinutesAlignmentCLI(t, caller, "minutes", "+list-shared", "--page-all", "--page-limit", "1")
		if err == nil || payload["complete"] != false || payload["nextToken"] != "n2" {
			t.Fatalf("limit payload=%#v err=%v", payload, err)
		}
	})

	t.Run("accessible cursor conflict", func(t *testing.T) {
		caller := &minutesE2ECaller{}
		if _, _, err := runMinutesAlignmentCLI(t, caller, "minutes", "+list-all", "--cursor", "n2", "--page-all"); err == nil || len(caller.counts) != 0 {
			t.Fatalf("cursor conflict err=%v calls=%#v", err, caller.counts)
		}
	})
}
