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
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
)

// identity-deregistry probe (Phase 1).
//
// These helpers test the hypothesis that command identity can be collected
// from live Cobra leaves carrying ContractFinal.Identity, producing a command
// spec set byte-equivalent to the reviewed schema_command_registry. They are
// additive: nothing here changes production behaviour. The comparison test
// (internal/app) gates Phase 2 on SourceHash equivalence.

// IdentityCollectionReport summarises a collection walk for the probe.
type IdentityCollectionReport struct {
	Leaves          int      // runnable leaves walked (hidden included)
	WithIdentity    int      // primary leaves carrying a ContractFinal.Identity
	HiddenPrimaries int      // collected primaries that are Hidden deprecated/migration shims
	Excluded        int      // leaves matched by reviewed exclusions
	NoIdentity      []string // leaves without Identity (alias / compatibility / deprecated; informational)
	MissingPrimary  []string // reviewed canonicals with no collected primary (the real gap)
}

// ReviewedCommandSpecs exposes the reviewed registry's command specs for the
// probe comparison. Production assembly continues to use the existing path.
func ReviewedCommandSpecs() ([]CommandSpec, error) {
	registry, err := loadReviewedCommandRegistry()
	if err != nil {
		return nil, err
	}
	return registry.Commands, nil
}

// BuildEffectiveFromSpecs wraps the shared indexing so collected and reviewed
// specs are normalised by the exact same rules before hashing.
func BuildEffectiveFromSpecs(specs []CommandSpec) (EffectiveCommandRegistry, error) {
	return newEffectiveCommandRegistry(specs)
}

// primaryCLIPathFromIdentity resolves the primary leaf path the way the
// registry does: PrimaryCLIPath wins, CLIPath fills when primary is empty.
func primaryCLIPathFromIdentity(id contract.ToolIdentitySpec) string {
	primary := strings.TrimSpace(id.PrimaryCLIPath)
	if primary == "" {
		primary = strings.TrimSpace(id.CLIPath)
	}
	return normalizeSchemaCLIPath(primary)
}

// walkIdentityLeaves visits every runnable leaf under cmd, INCLUDING Hidden
// ones. The production bind path (bindCommandRegistryPath →
// resolveExactCobraPath) resolves registry entries through raw Commands() with
// no availability filter, so hidden deprecated shims that still carry a
// reviewed registry entry and ContractFinal.Identity are part of the current
// effective surface. The collector must match that reachability to be
// byte-equivalent; filtering on IsAvailableCommand (as the public-surface
// completeness walk does) would silently drop them.
func walkIdentityLeaves(cmd *cobra.Command, fn func(*cobra.Command)) {
	if cmd == nil {
		return
	}
	if cmd.Runnable() && !cmd.HasSubCommands() {
		fn(cmd)
		return
	}
	for _, child := range cmd.Commands() {
		if child.Name() == "help" {
			continue
		}
		walkIdentityLeaves(child, fn)
	}
}

// CollectIdentitySpecs walks the public runnable leaves under root and
// synthesises a CommandSpec from each leaf's ContractFinal.Identity, matching
// the reviewed registry decode semantics (Source constant, public visibility,
// SourceProductID defaulting handled by the shared indexer).
func CollectIdentitySpecs(root *cobra.Command) ([]CommandSpec, IdentityCollectionReport, error) {
	report := IdentityCollectionReport{}
	if root == nil {
		return nil, report, fmt.Errorf("collect identity specs: root is nil")
	}
	exclusions, err := ReviewedRuntimeSchemaExclusions()
	if err != nil {
		return nil, report, err
	}
	excluded := make(map[string]bool, len(exclusions))
	for _, exclusion := range exclusions {
		path := normalizeSchemaCLIPath(exclusion.CLIPath)
		if path != "" {
			excluded[path] = true
		}
	}

	specs := []CommandSpec{}
	walkIdentityLeaves(root, func(leaf *cobra.Command) {
		report.Leaves++
		path := normalizeSchemaCLIPath(strings.Join(commandPathParts(leaf), " "))
		if path != "" && excluded[path] {
			report.Excluded++
			return
		}
		final, ok := contractfinal.RuntimeContractFinal(leaf)
		if !ok || final.Identity == nil {
			// Alias / compatibility / deprecated leaves carry no Identity of
			// their own; they resolve to a primary. Informational only.
			report.NoIdentity = append(report.NoIdentity, path)
			return
		}
		report.WithIdentity++
		if leaf.Hidden {
			report.HiddenPrimaries++
		}
		id := *final.Identity
		specs = append(specs, CommandSpec{
			CanonicalPath:   strings.TrimSpace(id.CanonicalPath),
			SourceProductID: strings.TrimSpace(id.SourceProductID),
			PrimaryCLIPath:  primaryCLIPathFromIdentity(id),
			Aliases:         append([]string(nil), id.Aliases...),
			Visibility:      SchemaVisibilityPublic,
			Source:          "reviewed_command_registry",
		})
	})
	sort.Slice(specs, func(i, j int) bool { return specs[i].CanonicalPath < specs[j].CanonicalPath })
	sort.Strings(report.NoIdentity)
	return specs, report, nil
}

// DiagnoseMissingPrimaries resolves each reviewed spec that has no collected
// primary and reports why: the reviewed primary CLI path may not exist as a
// Cobra leaf, the leaf may lack ContractFinal, or its declared canonical may
// drift from the reviewed one.
func DiagnoseMissingPrimaries(root *cobra.Command, missing []CommandSpec) []string {
	var diagnostics []string
	for _, spec := range missing {
		path := normalizeSchemaCLIPath(spec.PrimaryCLIPath)
		match, err := resolveExactCobraPath(root, path)
		if err != nil || match.Command == nil {
			diagnostics = append(diagnostics, fmt.Sprintf("%s: primary path %q has no Cobra leaf (%v)", spec.CanonicalPath, path, err))
			continue
		}
		final, ok := contractfinal.RuntimeContractFinal(match.Command)
		if !ok || final.Identity == nil {
			diagnostics = append(diagnostics, fmt.Sprintf("%s: leaf %q has no ContractFinal.Identity", spec.CanonicalPath, path))
			continue
		}
		declared := strings.TrimSpace(final.Identity.CanonicalPath)
		diagnostics = append(diagnostics, fmt.Sprintf("%s: leaf %q declares canonical %q (reviewed %q)", spec.CanonicalPath, path, declared, spec.CanonicalPath))
	}
	sort.Strings(diagnostics)
	return diagnostics
}

// CompareCommandSpecEquivalence returns a human-readable, deterministic diff
// between collected and reviewed specs keyed by canonical path. An empty slice
// means the two sets agree on every compared field.
func CompareCommandSpecEquivalence(collected, reviewed []CommandSpec) []string {
	byCanonical := func(specs []CommandSpec) map[string]CommandSpec {
		m := make(map[string]CommandSpec, len(specs))
		for _, spec := range specs {
			m[strings.TrimSpace(spec.CanonicalPath)] = spec
		}
		return m
	}
	collectedMap := byCanonical(collected)
	reviewedMap := byCanonical(reviewed)

	var problems []string
	for canonical := range reviewedMap {
		if _, present := collectedMap[canonical]; !present {
			problems = append(problems, fmt.Sprintf("missing from collected: %s", canonical))
		}
	}
	for canonical := range collectedMap {
		if _, present := reviewedMap[canonical]; !present {
			problems = append(problems, fmt.Sprintf("extra in collected (not reviewed): %s", canonical))
		}
	}
	for canonical, got := range collectedMap {
		want, present := reviewedMap[canonical]
		if !present {
			continue
		}
		if got.PrimaryCLIPath != want.PrimaryCLIPath {
			problems = append(problems, fmt.Sprintf("%s primary_cli_path: collected %q, reviewed %q", canonical, got.PrimaryCLIPath, want.PrimaryCLIPath))
		}
		if got.SourceProductID != want.SourceProductID {
			problems = append(problems, fmt.Sprintf("%s source_product_id: collected %q, reviewed %q", canonical, got.SourceProductID, want.SourceProductID))
		}
		if got.Source != want.Source {
			problems = append(problems, fmt.Sprintf("%s source: collected %q, reviewed %q", canonical, got.Source, want.Source))
		}
		if got.Visibility != want.Visibility {
			problems = append(problems, fmt.Sprintf("%s visibility: collected %q, reviewed %q", canonical, got.Visibility, want.Visibility))
		}
		if !stringSlicesEqualAsSet(got.Aliases, want.Aliases) {
			problems = append(problems, fmt.Sprintf("%s aliases: collected %v, reviewed %v", canonical, got.Aliases, want.Aliases))
		}
	}
	sort.Strings(problems)
	return problems
}
