// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agentmetadata

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/contractfinal"
	"github.com/spf13/cobra"
)

// RegistryProjection is the EffectiveCommandRegistry view required by Generate.
// Catalog assembly and the optional diagnostic Agent-metadata CLI both build
// this in-memory; neither path reads schema_agent_metadata/ from disk.
type RegistryProjection struct {
	ToolPaths          map[string]string
	CanonicalToolPaths map[string]string
	ProductIDs         map[string]bool
	Hash               string
	ToolCount          int
	Bound              cli.BoundCommandRegistry
}

// ProjectEffectiveRegistry builds the Generate projection from a public
// Effective registry. Keeping projection below the post-manual boundary
// prevents a base-registry allowlist from silently dropping reviewed
// manual-only commands.
func ProjectEffectiveRegistry(effective cli.EffectiveCommandRegistry) RegistryProjection {
	projection := RegistryProjection{
		ToolPaths:          map[string]string{},
		CanonicalToolPaths: map[string]string{},
		ProductIDs:         map[string]bool{},
		Hash:               effective.SourceHash(),
	}
	for _, command := range effective.Commands {
		if command.Visibility != cli.SchemaVisibilityPublic {
			continue
		}
		projection.ToolCount++
		primary := strings.TrimSpace(command.PrimaryCLIPath)
		projection.ToolPaths[primary] = primary
		projection.ToolPaths[command.CanonicalPath] = primary
		projection.CanonicalToolPaths[command.CanonicalPath] = primary
		if productID, _, ok := strings.Cut(command.CanonicalPath, "."); ok && strings.TrimSpace(productID) != "" {
			projection.ProductIDs[strings.TrimSpace(productID)] = true
		}
		for _, alias := range command.Aliases {
			if alias = strings.TrimSpace(alias); alias != "" {
				projection.ToolPaths[alias] = primary
			}
		}
	}
	return projection
}

// SelectionHintCoverageTools returns the canonical tools that still need a
// selection/ hint row. ContractFinal-declared leaves are exempt because their
// selection fields live on the leaf declaration.
func SelectionHintCoverageTools(projection RegistryProjection) map[string]bool {
	expectedTools := make(map[string]bool, len(projection.CanonicalToolPaths))
	for canonical := range projection.CanonicalToolPaths {
		expectedTools[canonical] = true
	}
	for canonical := range expectedTools {
		bound, ok := projection.Bound.ByCanonical[canonical]
		if ok && contractfinal.HasRuntimeContractFinal(bound.PrimaryCommand) {
			delete(expectedTools, canonical)
		}
	}
	return expectedTools
}

// SelectionHintCoverageProducts returns product IDs that still need a
// selection/ products{} row. ProductDecl-registered products are exempt
// because their routing prose lives on the in-code declaration.
func SelectionHintCoverageProducts(projection RegistryProjection) map[string]bool {
	return selectionHintCoverageProducts(projection.ProductIDs)
}

// ValidateSelectionCoverage requires every projected product to have
// ProductDecl and every projected tool to have ContractFinal. Residual
// schema_hints/ HintFiles are not accepted.
func ValidateSelectionCoverage(projection RegistryProjection) error {
	expectedTools := SelectionHintCoverageTools(projection)
	expectedProducts := SelectionHintCoverageProducts(projection)
	if !selectionHintCoverageRequired(expectedProducts, expectedTools) {
		return nil
	}
	missingProducts := make([]string, 0)
	for productID, include := range expectedProducts {
		if include {
			missingProducts = append(missingProducts, productID)
		}
	}
	missingTools := make([]string, 0)
	for tool, include := range expectedTools {
		if include {
			missingTools = append(missingTools, tool)
		}
	}
	sort.Strings(missingProducts)
	sort.Strings(missingTools)
	return fmt.Errorf("ProductDecl/ContractFinal selection coverage incomplete: missing_products=%v missing_tools=%v", missingProducts, missingTools)
}

// ValidateSelectionHints is a compatibility wrapper that ignores HintsDir and
// enforces ContractFinal/ProductDecl coverage only.
func ValidateSelectionHints(_ string, _ string, projection RegistryProjection) error {
	return ValidateSelectionCoverage(projection)
}

var (
	pipelineBuildEffectiveRegistry = cli.BuildEffectiveCommandRegistry
	pipelineBindEffectiveRegistry  = cli.BindEffectiveCommandRegistry
	pipelineGenerateMetadata       = Generate
)

func selectionHintCoverageRequired(expectedProducts, expectedTools map[string]bool) bool {
	for _, include := range expectedProducts {
		if include {
			return true
		}
	}
	for _, include := range expectedTools {
		if include {
			return true
		}
	}
	return false
}

// GenerateFromCommandRoot is the Catalog in-memory Agent-metadata pipeline.
// It binds the EffectiveCommandRegistry, validates that ProductDecl/ContractFinal
// cover selection, and Generate()s metadata without reading or writing
// schema_agent_metadata/ or schema_hints/.
func GenerateFromCommandRoot(rootPath string, commandRoot *cobra.Command, opts Options) (File, Stats, RegistryProjection, error) {
	if commandRoot == nil {
		return File{}, Stats{}, RegistryProjection{}, fmt.Errorf("schema source root is nil")
	}
	rootPath = strings.TrimSpace(rootPath)
	if rootPath == "" {
		rootPath = "."
	}
	effective, err := pipelineBuildEffectiveRegistry(commandRoot)
	if err != nil {
		return File{}, Stats{}, RegistryProjection{}, fmt.Errorf("build effective CommandRegistry for Agent metadata: %w", err)
	}
	bound, err := pipelineBindEffectiveRegistry(commandRoot, effective)
	if err != nil {
		return File{}, Stats{}, RegistryProjection{}, fmt.Errorf("bind effective CommandRegistry for Agent metadata: %w", err)
	}
	projection := ProjectEffectiveRegistry(effective)
	projection.Bound = bound

	opts.Root = rootPath
	if strings.TrimSpace(opts.SkillPath) == "" {
		opts.SkillPath = "skills/mono/SKILL.md"
	}
	if strings.TrimSpace(opts.ProductsDir) == "" {
		opts.ProductsDir = "skills/mono/references/products"
	}
	if strings.TrimSpace(opts.IntentGuidePath) == "" {
		opts.IntentGuidePath = "skills/mono/references/intent-guide.md"
	}
	// HintsDir must stay empty: schema_hints/ is retired and non-empty values fail closed.
	if strings.TrimSpace(opts.InterfaceMetadataPath) == "" {
		opts.InterfaceMetadataPath = "internal/cli/schema_mcp_metadata.json"
	}
	if opts.MaxExamples <= 0 {
		opts.MaxExamples = 2
	}
	if opts.MaxInterfaceSummaryRunes <= 0 {
		opts.MaxInterfaceSummaryRunes = 120
	}
	opts.ToolPaths = projection.ToolPaths
	opts.CanonicalToolPaths = projection.CanonicalToolPaths
	opts.BoundCommands = projection.Bound
	opts.ProductIDs = projection.ProductIDs
	opts.SurfaceHash = projection.Hash
	opts.SurfaceToolCount = projection.ToolCount

	if err := ValidateSelectionCoverage(projection); err != nil {
		return File{}, Stats{}, RegistryProjection{}, err
	}
	metadata, stats, err := pipelineGenerateMetadata(opts)
	if err != nil {
		return File{}, Stats{}, RegistryProjection{}, fmt.Errorf("generate in-memory Agent metadata: %w", err)
	}
	return metadata, stats, projection, nil
}

func resolvePipelineRootPath(root, path string) string {
	path = strings.TrimSpace(path)
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
