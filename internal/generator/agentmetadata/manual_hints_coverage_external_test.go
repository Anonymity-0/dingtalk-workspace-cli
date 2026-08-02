// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package agentmetadata_test

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/app"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/generator/agentmetadata"
)

// Kept in package agentmetadata_test so importing app (and its ProductDecl
// registrations) cannot pollute fixture-based generateFromSources tests in
// package agentmetadata.
func TestCrossPlatformCoverageValidateSelectionAuthoringContractsRealBoundRegistry(t *testing.T) {
	root := app.NewSchemaSourceRootCommand()
	effective, err := cli.BuildEffectiveCommandRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := cli.BindEffectiveCommandRegistry(root, effective)
	if err != nil {
		t.Fatal(err)
	}
	projection := agentmetadata.ProjectEffectiveRegistry(effective)
	if err := agentmetadata.ValidateSelectionAuthoringContractsForTest(agentmetadata.Options{
		CanonicalToolPaths: projection.CanonicalToolPaths,
		ProductIDs:         projection.ProductIDs,
		BoundCommands:      bound,
	}); err != nil {
		t.Fatalf("real bound registry: %v", err)
	}
}
