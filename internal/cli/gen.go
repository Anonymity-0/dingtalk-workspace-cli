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

// gen.go is the single entry point for reviewed CLI asset generation. It isolates
// all //go:generate pragmas from business code so that:
//   - schema_catalog.go contains only types + embed.
//   - Generation is a standalone process (make generate-schema triggers this).
//   - The authored-input → generated-output contract is documented in one place.
//
// Generation inputs (authored, reviewed):
//   1. schema_command_registry/             identity (canonical/aliases/navigation)
//   2. ProductDecl + leaf ContractFinal    Agent routing / selection prose
//   3. schema_mcp_metadata.json            MCP server tool definitions
//   4. schema_parameter_bindings.json      parameter type/property mappings
//   5. param_concepts.json + schema       reviewed parameter synonym policy
//   6. cobra command tree (Go runtime)     flags/usage/required (reflected)
//
// schema_hints/selection/ and schema_hints/metadata/ are retired (directories
// may be absent). index.selection / index.metadata may be omitted or {}.
// schema_agent_metadata/ is retired: Catalog generation injects Agent metadata
// in-memory and does not write or embed that intermediate JSON directory.
//
// Generation outputs (embedded at build):
//   - schema_catalog/                      per-product catalog shards
//   - param_aliases_generated.go           per-command parameter normalization

package cli

//go:generate go run -a ../generator/cmd_schema_catalog -root ../.. -output schema_catalog
//go:generate go run ../generator/cmd_param_aliases -root ../.. -output param_aliases_generated.go
