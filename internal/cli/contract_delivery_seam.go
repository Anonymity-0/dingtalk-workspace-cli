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
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

// Delivery-seam helpers that must live in cli because they emit Cobra
// annotations (AnnotateRuntime*). Types and registries live in
// corecmd/contract; callers author contract.SafetySpec / contract.ParamDecl /
// contract.ContractFinalPayload directly — there is no cli type alias layer.

// RegisterRuntimeContractFinal stores the typed final Schema overlay and marks
// the leaf with the runtime contract annotation for delivery consumers.
func RegisterRuntimeContractFinal(cmd *cobra.Command, payload contract.ContractFinalPayload) {
	if cmd == nil {
		return
	}
	AnnotateRuntimeContract(cmd)
	contract.RegisterRuntimeContractFinal(cmd, payload)
}

// ApplyParamDecls emits parameter declarations as dws.schema.* annotations on the
// command's flags. Called at assembly time when all flags exist on the tree.
// Each non-blank ParamDecl.Name must resolve to an existing Cobra flag;
// unknown names fail closed so typos cannot silently drop during generation.
func ApplyParamDecls(cmd *cobra.Command, decls []contract.ParamDecl) error {
	if cmd == nil || len(decls) == 0 {
		return nil
	}
	for _, p := range decls {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		if runtimeCommandFlag(cmd, name) == nil {
			return fmt.Errorf("ParamDecl %q references unknown flag on %q", name, cmd.CommandPath())
		}
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
	return nil
}

func resolvedFieldProvenance(value any, source, sourceRef, precedence, resolution, reviewReason string) contract.FieldProvenance {
	return contract.ResolvedFieldProvenance(value, source, sourceRef, precedence, resolution, reviewReason)
}
