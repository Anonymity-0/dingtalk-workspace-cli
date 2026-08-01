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

// Production framework code must register ContractFinal through the cli
// delivery seam (annotate + store), not by calling the contract store helper
// directly.
func TestAttachContractUsesCLIRegisterSeam(t *testing.T) {
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
	if !strings.Contains(body, "cli.RegisterRuntimeContractFinal(") {
		t.Fatal("AttachContract/New must call cli.RegisterRuntimeContractFinal")
	}
	// Disallow the dual-track pattern: annotate then contract.Register in corecmd.
	if strings.Contains(body, "contract.RegisterRuntimeContractFinal(") {
		t.Fatal("corecmd must not call contract.RegisterRuntimeContractFinal directly; use cli seam")
	}
}
