// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package smart

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

func TestCrossPlatformCoverageMinutesResultContracts(t *testing.T) {
	transcriptResult, err := contract.NormalizeResultSpec(Transcript.Contract.Result, "minutes.shortcut_transcript")
	if err != nil {
		t.Fatalf("normalize transcript result: %v", err)
	}
	if transcriptResult == nil {
		t.Fatal("transcript result contract is missing")
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
