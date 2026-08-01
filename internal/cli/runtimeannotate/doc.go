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

// Package runtimeannotate owns Cobra dws.schema.* annotation writers and the
// RuntimeSchemaConstraints helpers used by the command framework.
//
// Package boundary:
//
//   - Types / DTO / ProductDecl → internal/corecmd/contract
//   - AnnotateRuntime* writers (this package) — no Catalog / go:embed
//   - ContractFinal cobra store + Register seam → internal/cli/contractfinal
//   - Catalog assembly / ResolveMeta / go:embed → internal/cli (root)
//
// Dependency direction: corecmd and cli/contractfinal import this package;
// this package must never import internal/cli (root) or contractfinal.
package runtimeannotate
