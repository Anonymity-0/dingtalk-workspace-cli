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

// Package contract owns the command-framework declaration registries and the
// leaf/product types that form ContractFinal pass-through.
//
// This package is the sole definition point for:
//   - ContractFinalPayload (+ registry store)
//   - ProductDecl (+ registry)
//   - SafetySpec / SelectionSpec / InterfaceSpec / DryRunSpec / identity /
//     positionals / ParamDecl
//   - FieldProvenance / FieldCandidateProvenance
//
// Package boundary (corecmd → cli seam):
//
//   - Types and registries → corecmd/contract (this package). Callers author
//     contract.SafetySpec / contract.ParamDecl / contract.ProductDecl /
//     contract.ContractFinalPayload / contract.InterfaceSpec directly.
//   - Authoring wrapper → corecmd.ContractDecl (leaf-facing; nested fields are
//     these contract types). Name is ContractDecl, not SchemaDecl: "Schema" in
//     this repo means Catalog / ToolSpec delivery, not the author declaration.
//   - Cobra annotation + store seam → internal/cli.RegisterRuntimeContractFinal
//     (AnnotateRuntimeContract + RegisterRuntimeContractFinal). Production
//     framework code (corecmd.New / AttachContract) must use that seam and must
//     not call this package's store helper directly.
//   - Catalog assembly / ResolveMeta / go:embed → internal/cli (delivery
//     boundary; not moved into contract).
//
// Authoring path: corecmd.ContractDecl → ContractFinalPayload (via cli seam);
// ProductDecl for product-level Agent routing. Provenance stamp for declared
// leaf Safety remains "corecmd.contract".
package contract
