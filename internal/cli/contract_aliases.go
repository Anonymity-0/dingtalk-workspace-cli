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

	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

// Leaf Schema contract types are defined in internal/corecmd/contract.
// Aliases keep existing cli.* call sites compiling while assembly migrates.

type (
	ToolIdentitySpec         = contract.ToolIdentitySpec
	RuntimeSchemaPositional  = contract.RuntimeSchemaPositional
	DryRunSpec               = contract.DryRunSpec
	SafetySpec               = contract.SafetySpec
	InterfaceRefSpec         = contract.InterfaceRefSpec
	InterfaceSpec            = contract.InterfaceSpec
	SelectionSpec            = contract.SelectionSpec
	FieldProvenance          = contract.FieldProvenance
	FieldCandidateProvenance = contract.FieldCandidateProvenance
	ParamDecl                = contract.ParamDecl
	ContractFinalPayload     = contract.ContractFinalPayload
	ProductSelectionDecl     = contract.ProductSelectionDecl
	ProductDecl              = contract.ProductDecl
)

const (
	DryRunPreviewInvocation = contract.DryRunPreviewInvocation
	DryRunPreviewRequest    = contract.DryRunPreviewRequest
	DryRunPreviewPlan       = contract.DryRunPreviewPlan
	DryRunPreviewDiff       = contract.DryRunPreviewDiff

	InterfaceModeMCP       = contract.InterfaceModeMCP
	InterfaceModeLocal     = contract.InterfaceModeLocal
	InterfaceModeComposite = contract.InterfaceModeComposite
	InterfaceAvailable     = contract.InterfaceAvailable
	InterfaceUnavailable   = contract.InterfaceUnavailable

	ProductDeclProvenanceSource = contract.ProductDeclProvenanceSource
	ProductDeclSourceRef        = contract.ProductDeclSourceRef
)

// RegisterRuntimeContractFinal stores the typed final Schema overlay and marks
// the leaf with the runtime contract annotation for delivery consumers.
func RegisterRuntimeContractFinal(cmd *cobra.Command, payload ContractFinalPayload) {
	if cmd == nil {
		return
	}
	AnnotateRuntimeContract(cmd)
	contract.RegisterRuntimeContractFinal(cmd, payload)
}

// RuntimeContractFinal returns the registered final Schema overlay (read-only).
func RuntimeContractFinal(cmd *cobra.Command) (ContractFinalPayload, bool) {
	return contract.RuntimeContractFinal(cmd)
}

// HasRuntimeContractFinal reports whether the leaf has a registered final overlay.
func HasRuntimeContractFinal(cmd *cobra.Command) bool {
	return contract.HasRuntimeContractFinal(cmd)
}

// ClearRuntimeContractFinalForTest removes a registration (tests only).
func ClearRuntimeContractFinalForTest(cmd *cobra.Command) {
	contract.ClearRuntimeContractFinalForTest(cmd)
}

// storeRuntimeContractFinalRawForTest injects a raw map value (tests only).
func storeRuntimeContractFinalRawForTest(cmd *cobra.Command, raw any) {
	contract.StoreRuntimeContractFinalRawForTest(cmd, raw)
}

// RegisterProductDecl stores a product-level routing declaration.
func RegisterProductDecl(decl ProductDecl) {
	contract.RegisterProductDecl(decl)
}

// LookupProductDecl returns the registered product declaration, if any.
func LookupProductDecl(productID string) (ProductDecl, bool) {
	return contract.LookupProductDecl(productID)
}

// HasProductDecl reports whether product-level routing is declared in code.
func HasProductDecl(productID string) bool {
	return contract.HasProductDecl(productID)
}

// RegisteredProductDeclIDs returns sorted product IDs with an in-code Decl.
func RegisteredProductDeclIDs() []string {
	return contract.RegisteredProductDeclIDs()
}

// ClearProductDeclForTest removes a registration (tests only).
func ClearProductDeclForTest(productID string) {
	contract.ClearProductDeclForTest(productID)
}

// ProductSelectionFromDecl projects a ProductDecl into SelectionSpec plus
// contract_final FieldProvenance for ProductSpec assembly.
func ProductSelectionFromDecl(decl ProductDecl) (SelectionSpec, map[string]FieldProvenance) {
	return contract.ProductSelectionFromDecl(decl)
}

// ApplyParamDecls emits parameter declarations as dws.schema.* annotations on the
// command's flags. Called at assembly time when all flags exist on the tree.
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

func resolvedFieldProvenance(value any, source, sourceRef, precedence, resolution, reviewReason string) FieldProvenance {
	return contract.ResolvedFieldProvenance(value, source, sourceRef, precedence, resolution, reviewReason)
}
