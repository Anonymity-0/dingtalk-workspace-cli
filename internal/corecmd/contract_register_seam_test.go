// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package corecmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Production framework code must register ContractFinal through the
// contractfinal annotate+store seam, not by importing the cli delivery root.
func TestAttachContractUsesContractFinalRegisterSeam(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src := filepath.Join(filepath.Dir(thisFile), "corecmd.go")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read corecmd.go: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "contractfinal.RegisterRuntimeContractFinal(") {
		t.Fatal("AttachContract/New must call contractfinal.RegisterRuntimeContractFinal")
	}
	if strings.Contains(body, `"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"`) {
		t.Fatal("corecmd must not import internal/cli delivery root")
	}
	if strings.Contains(body, "cli.RegisterRuntimeContractFinal(") {
		t.Fatal("corecmd must not call cli.RegisterRuntimeContractFinal; use contractfinal seam")
	}
}
