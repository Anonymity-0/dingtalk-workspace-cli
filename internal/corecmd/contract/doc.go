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

// Package contract owns the command-framework Schema declaration registries
// and the leaf/product types that form ContractFinal pass-through.
//
// This package is the sole definition point for:
//   - ContractFinalPayload (+ registry)
//   - ProductDecl (+ registry)
//   - SafetySpec / SelectionSpec / InterfaceSpec / DryRunSpec / identity /
//     positionals / ParamDecl
//   - FieldProvenance / FieldCandidateProvenance
//
// Package boundary (corecmd → cli seam):
//
//   - Types and registries → corecmd/contract (this package). Callers author
//     contract.SafetySpec / contract.ParamDecl / contract.ProductDecl /
//     contract.ContractFinalPayload directly; there is no cli type-alias layer.
//   - Cobra annotation seam (AnnotateRuntime*) → internal/cli. Framework code
//     (corecmd.New / AttachSchema) may call those helpers to embed
//     dws.schema.* facts; cli must not redefine the contract types.
//   - Delivery helpers that must emit annotations
//     (cli.RegisterRuntimeContractFinal, cli.ApplyParamDecls) stay in cli but
//     accept contract.* payloads.
//   - Catalog assembly / ResolveMeta / go:embed → internal/cli (delivery
//     boundary; not moved into contract).
//
// Authoring path: corecmd.SchemaDecl → ContractFinalPayload; ProductDecl for
// product-level Agent routing. Provenance stamp for declared leaf Safety
// remains "corecmd.contract".
package contract
