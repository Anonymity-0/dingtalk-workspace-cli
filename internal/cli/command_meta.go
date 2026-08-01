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

// command_meta.go provides the unified metadata consumption API. All runtime
// consumers (help, schema, agent selection, skill generation) call ResolveMeta
// to get a CommandMeta struct — one function, one struct, no need to know which
// of the 6 generation layers a field comes from.
//
// This is the "simple consumption" half of the generation/consumption split:
//   - Generation (gen.go + internal/generator/): 6 inputs → catalog snapshot
//     + schema_meta_index.json (CommandMeta summary).
//   - Consumption (this file): meta index → ResolveMeta → CommandMeta.
//     Full embeddedSchemaCatalog() is reserved for dws schema / ToolSpec paths.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// CommandMeta is the complete runtime metadata view for a single command.
// Consumers read this struct; they never touch the raw catalog maps.
type CommandMeta struct {
	Identity  CommandIdentity
	Safety    CommandSafety
	Selection CommandSelection
}

// CommandIdentity is the stable identity of a command.
type CommandIdentity struct {
	CLIPath   string   // "dev app delete"
	Canonical string   // "dev.delete_dev_app"
	Aliases   []string // ["search", ...]
	ProductID string   // "devapp"
	Title     string   // one-line description
}

// CommandSelection is the agent-facing selection metadata.
type CommandSelection struct {
	AgentSummary string
	UseWhen      []string
	AvoidWhen    []string
	Examples     []string
}

var (
	metaByCLIPathOnce sync.Once
	metaByCLIPath     map[string]CommandMeta
)

// initMetaByCLIPath builds the cli_path → CommandMeta lookup from the embedded
// CommandMeta summary index. It does not decode the full schema_catalog/.
// Decode failure is fail-closed: the error is retained and ResolveMeta panics
// rather than serving an empty map that would silently hide help Safety.
func initMetaByCLIPath() {
	runtimeEmbeddedSchemaMetaIndexLazyCount.Add(1)
	lookup, err := decodeSchemaMetaIndexLookup(embeddedSchemaMetaIndexJSON)
	if err != nil {
		runtimeEmbeddedSchemaMetaIndexErr = err
		metaByCLIPath = nil
		return
	}
	metaByCLIPath = lookup
}

// panicIfMetaIndexUnusable fails closed when the embedded CommandMeta summary
// could not be decoded. Callers must not treat this as "command missing".
func panicIfMetaIndexUnusable(err error) {
	if err == nil {
		return
	}
	panic(fmt.Sprintf("embedded schema_meta_index.json is unusable: %v", err))
}

// buildMetaByCLIPath constructs the lookup from a loaded catalog.
// Prefer the typed Registry (production cold-start path). Fall back to
// Snapshot.Tools maps for unit tests that synthesize untyped fixtures.
func buildMetaByCLIPath(loaded loadedSchemaCatalog) map[string]CommandMeta {
	if len(loaded.Registry.Products) > 0 {
		return buildMetaByCLIPathFromRegistry(loaded.Registry)
	}
	return buildMetaByCLIPathFromSnapshotTools(loaded.Snapshot.Tools)
}

func buildMetaByCLIPathFromRegistry(registry SchemaRegistry) map[string]CommandMeta {
	lookup := make(map[string]CommandMeta)
	metas := make([]CommandMeta, 0, 64)
	for _, product := range registry.Products {
		for _, tool := range product.Tools {
			cliPath := strings.TrimSpace(tool.Identity.CLIPath)
			if cliPath == "" {
				continue
			}
			meta := CommandMeta{
				Identity: CommandIdentity{
					CLIPath:   cliPath,
					Canonical: tool.Identity.CanonicalPath,
					Aliases:   append([]string(nil), tool.Identity.Aliases...),
					ProductID: tool.Identity.ProductID,
					Title:     tool.Title,
				},
				Safety: CommandSafety{
					Effect:       tool.Safety.Effect,
					Risk:         tool.Safety.Risk,
					Confirmation: tool.Safety.Confirmation,
					Idempotency:  tool.Safety.Idempotency,
				},
				Selection: CommandSelection{
					AgentSummary: tool.Selection.AgentSummary,
					UseWhen:      append([]string(nil), tool.Selection.UseWhen...),
					AvoidWhen:    append([]string(nil), tool.Selection.AvoidWhen...),
					Examples:     append([]string(nil), tool.Selection.Examples...),
				},
			}
			lookup[cliPath] = meta
			metas = append(metas, meta)
		}
	}
	return registerCommandMetaAliases(lookup, metas)
}

func buildMetaByCLIPathFromSnapshotTools(tools map[string]map[string]any) map[string]CommandMeta {
	lookup := make(map[string]CommandMeta)
	if tools == nil {
		return lookup
	}
	metas := make([]CommandMeta, 0, len(tools))
	for _, tool := range tools {
		cliPath := schemaString(tool["cli_path"])
		if cliPath == "" {
			continue
		}
		meta := CommandMeta{
			Identity: CommandIdentity{
				CLIPath:   cliPath,
				Canonical: schemaString(tool["canonical_path"]),
				Aliases:   schemaStringSlice(tool["aliases"]),
				ProductID: schemaString(tool["product_id"]),
				Title:     schemaString(tool["title"]),
			},
			Safety: CommandSafety{
				Effect:       schemaString(tool["effect"]),
				Risk:         schemaString(tool["risk"]),
				Confirmation: schemaString(tool["confirmation"]),
				Idempotency:  schemaString(tool["idempotency"]),
			},
			Selection: CommandSelection{
				AgentSummary: schemaString(tool["agent_summary"]),
				UseWhen:      schemaStringSlice(tool["use_when"]),
				AvoidWhen:    schemaStringSlice(tool["avoid_when"]),
				Examples:     schemaStringSlice(tool["examples"]),
			},
		}
		lookup[cliPath] = meta
		metas = append(metas, meta)
	}
	return registerCommandMetaAliases(lookup, metas)
}

// registerCommandMetaAliases fills compat alias paths. Primary paths always
// win; alias-vs-alias collisions resolve to the owner with the
// lexicographically smallest primary cli_path (map iteration is unstable).
func registerCommandMetaAliases(lookup map[string]CommandMeta, metas []CommandMeta) map[string]CommandMeta {
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Identity.CLIPath < metas[j].Identity.CLIPath
	})
	for _, meta := range metas {
		for _, alias := range meta.Identity.Aliases {
			alias = strings.TrimSpace(alias)
			if alias == "" || alias == meta.Identity.CLIPath {
				continue
			}
			if _, exists := lookup[alias]; !exists {
				lookup[alias] = meta
			}
		}
	}
	return lookup
}

// ResolveMeta returns the complete metadata for a command identified by its CLI
// path (e.g. "dev app delete") or one of its compat aliases (e.g. "report list"
// for "report inbox list"). Returns ok=false for commands not in the embedded
// meta index (utility commands, hidden commands, shortcuts).
//
// ResolveMeta reads schema_meta_index.json only; it never triggers the full
// embeddedSchemaCatalog() decode used by dws schema / --all. A corrupt or
// undecodable meta index panics (fail-closed) so help Safety cannot silently
// disappear behind an empty lookup.
func ResolveMeta(cliPath string) (CommandMeta, bool) {
	metaByCLIPathOnce.Do(initMetaByCLIPath)
	panicIfMetaIndexUnusable(runtimeEmbeddedSchemaMetaIndexErr)
	m, ok := metaByCLIPath[strings.TrimSpace(cliPath)]
	return m, ok
}
