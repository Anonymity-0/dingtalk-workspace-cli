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
	"strings"
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
	Parameters  []ParamDecl
	Safety      *SafetySpec
	DryRun      *DryRunSpec
	Interface   *InterfaceSpec
	Selection   *SelectionSpec
	Identity    *ToolIdentitySpec
}

var contractFinalByCommand sync.Map // *cobra.Command → *ContractFinalPayload

// ParamDecl is one parameter-level Schema fact declared on a command. It is
// stored at DeclareLeafMetadata time and applied as annotations at assembly
// time, when all flags are guaranteed to exist on the fully-built command tree.
type ParamDecl struct {
	Name          string
	Property      string
	Required      *bool
	InterfaceType string
	Description   string
	RequiredWhen  string
	Enum          []string
}

// ApplyParamDecls emits parameter declarations as dws.schema.* annotations on the
// command's flags. Called at assembly time (runtimeToolSpecFromContractFinal)
// when all flags exist on the fully-built command tree. The decls come from
// the ContractFinalPayload, so no separate storage is needed.
func ApplyParamDecls(cmd *cobra.Command, decls []ParamDecl) {
	if cmd == nil || len(decls) == 0 {
		return
	}
	for _, p := range decls {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		if prop := strings.TrimSpace(p.Property); prop != "" {
			AnnotateRuntimeFlagProperty(cmd, name, prop)
		}
		if p.Required != nil {
			AnnotateRuntimeFlagRequiredValue(cmd, name, *p.Required)
		}
		if it := strings.TrimSpace(p.InterfaceType); it != "" {
			AnnotateRuntimeFlagInterfaceType(cmd, name, it)
		}
		if desc := strings.TrimSpace(p.Description); desc != "" {
			AnnotateRuntimeFlagDescription(cmd, name, desc)
		}
		if rw := strings.TrimSpace(p.RequiredWhen); rw != "" {
			AnnotateRuntimeFlagRequiredWhen(cmd, name, rw)
		}
		if len(p.Enum) > 0 {
			AnnotateRuntimeFlagEnum(cmd, name, p.Enum...)
		}
	}
}

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
