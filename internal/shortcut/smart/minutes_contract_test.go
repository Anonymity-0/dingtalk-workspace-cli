// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package smart

import (
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestCrossPlatformCoverageMinutesResultContracts(t *testing.T) {
	if Transcript.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("transcript rollout=%q, want unified_active", Transcript.OutputRollout)
	}
	transcriptResult, err := contract.NormalizeResultSpec(Transcript.Contract.Result, "minutes.shortcut_transcript")
	if err != nil {
		t.Fatalf("normalize transcript result: %v", err)
	}
	if transcriptResult == nil {
		t.Fatal("transcript result contract is missing")
	}
	if strings.Contains(string(transcriptResult.DataSchema), `"nextToken"`) {
		t.Fatal("transcript Result data_schema leaked pagination nextToken")
	}
	pagination, err := contract.NormalizePaginationSpec(Transcript.Contract.Pagination, "minutes.shortcut_transcript")
	if err != nil {
		t.Fatalf("normalize transcript pagination: %v", err)
	}
	if pagination == nil || pagination.CursorParameter != "cursor" {
		t.Fatalf("transcript pagination = %#v", pagination)
	}
	if MinutesDetail.Contract.Result == nil {
		t.Fatal("detail result contract is missing")
	}
	if _, err := contract.NormalizeResultSpec(MinutesDetail.Contract.Result, "minutes.shortcut_detail"); err != nil {
		t.Fatalf("normalize detail result: %v", err)
	}
}
