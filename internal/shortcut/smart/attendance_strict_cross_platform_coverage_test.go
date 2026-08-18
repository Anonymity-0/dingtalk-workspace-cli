// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package smart

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestCrossPlatformCoverageSmartAttendanceContractsAreUnifiedAndTyped(t *testing.T) {
	for _, declaration := range []struct {
		name      string
		rollout   output.RolloutState
		hasResult bool
	}{
		{MyAttendance.Command, MyAttendance.OutputRollout, MyAttendance.Contract.Result != nil},
		{ThisMonthAttendance.Command, ThisMonthAttendance.OutputRollout, ThisMonthAttendance.Contract.Result != nil},
	} {
		if declaration.rollout != output.RolloutUnifiedActive {
			t.Errorf("%s rollout = %q, want unified_active", declaration.name, declaration.rollout)
		}
		if !declaration.hasResult {
			t.Errorf("%s must publish a Result declaration", declaration.name)
		}
	}
}
