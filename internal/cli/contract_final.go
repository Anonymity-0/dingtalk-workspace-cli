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

package cli

import (
	"sync"

	"github.com/spf13/cobra"
)

// ContractFinalPayload is the Contract-authored final Schema leaf overlay.
// Registered in-process by the framework; Schema assembly reads it as
// pass-through. No JSON bridge. Treat as read-only after Register.
type ContractFinalPayload struct {
	Title       string
	Description string
	Positionals []RuntimeSchemaPositional
	Safety      *SafetySpec
	DryRun      *DryRunSpec
	Interface   *InterfaceSpec
	Selection   *SelectionSpec
	Identity    *ToolIdentitySpec
}

var contractFinalByCommand sync.Map // *cobra.Command → *ContractFinalPayload

// RegisterRuntimeContractFinal stores the typed final Schema overlay for a leaf.
// Light runtime write: one map store, no JSON, no deep clone.
func RegisterRuntimeContractFinal(cmd *cobra.Command, payload ContractFinalPayload) {
	if cmd == nil {
		return
	}
	AnnotateRuntimeContract(cmd)
	p := payload
	contractFinalByCommand.Store(cmd, &p)
}

// RuntimeContractFinal returns the registered final Schema overlay (read-only).
func RuntimeContractFinal(cmd *cobra.Command) (ContractFinalPayload, bool) {
	if cmd == nil {
		return ContractFinalPayload{}, false
	}
	raw, ok := contractFinalByCommand.Load(cmd)
	if !ok {
		return ContractFinalPayload{}, false
	}
	p, ok := raw.(*ContractFinalPayload)
	if !ok || p == nil {
		return ContractFinalPayload{}, false
	}
	return *p, true
}

// HasRuntimeContractFinal reports whether the leaf has a registered final overlay.
func HasRuntimeContractFinal(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	_, ok := contractFinalByCommand.Load(cmd)
	return ok
}

// ClearRuntimeContractFinalForTest removes a registration (tests only).
func ClearRuntimeContractFinalForTest(cmd *cobra.Command) {
	if cmd != nil {
		contractFinalByCommand.Delete(cmd)
	}
}
