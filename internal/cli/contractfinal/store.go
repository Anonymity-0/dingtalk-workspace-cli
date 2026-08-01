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

package contractfinal

import (
	"sync"

	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/runtimeannotate"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

var contractFinalByCommand sync.Map // *cobra.Command → *contract.ContractFinalPayload

// RegisterRuntimeContractFinal annotates dws.schema.contract then stores the
// typed final Schema overlay. This is the atomic annotate+store implementation.
//
// cli.RegisterRuntimeContractFinal is the delivery-facing name for product code;
// corecmd.AttachContract calls this function directly.
func RegisterRuntimeContractFinal(cmd *cobra.Command, payload contract.ContractFinalPayload) {
	if cmd == nil {
		return
	}
	runtimeannotate.AnnotateRuntimeContract(cmd)
	p := payload
	contractFinalByCommand.Store(cmd, &p)
}

// RuntimeContractFinal returns the registered final Schema overlay (read-only).
func RuntimeContractFinal(cmd *cobra.Command) (contract.ContractFinalPayload, bool) {
	if cmd == nil {
		return contract.ContractFinalPayload{}, false
	}
	raw, ok := contractFinalByCommand.Load(cmd)
	if !ok {
		return contract.ContractFinalPayload{}, false
	}
	p, ok := raw.(*contract.ContractFinalPayload)
	if !ok || p == nil {
		return contract.ContractFinalPayload{}, false
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

// StoreRuntimeContractFinalRawForTest injects a raw map value (tests only).
func StoreRuntimeContractFinalRawForTest(cmd *cobra.Command, raw any) {
	if cmd != nil {
		contractFinalByCommand.Store(cmd, raw)
	}
}
