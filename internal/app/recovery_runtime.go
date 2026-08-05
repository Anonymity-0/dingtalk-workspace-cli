package app

import (
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/executor"
)

// captureRuntimeFailure previously persisted a recovery snapshot for
// `dws recovery`. The recovery CLI is removed; keep a no-op seam so the
// runner failure paths stay stable without reintroducing that package.
func captureRuntimeFailure(_ executor.Invocation, _, _ error) {}
