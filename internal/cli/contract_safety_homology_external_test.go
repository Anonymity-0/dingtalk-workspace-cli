// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli_test

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/app"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
)

// TestUserRequiredSafetyHomologyWithRuntimeGate proves Catalog Safety and the
// executable confirmation gate share one source for every live user_required leaf:
//
//  1. ContractFinal.Safety.Confirmation == live ToolSpec.Confirmation
//  2. Runtime gate is installed: DeclareLeafMetadata (HasContractConfirmSafety),
//     Sheet protect marker, or framework NewCommand/Shortcut RunE (verified by
//     closed-stdin Execute → confirmation_required / 用户取消了操作 without --yes)
func TestUserRequiredSafetyHomologyWithRuntimeGate(t *testing.T) {
	root := app.NewSchemaSourceRootCommand()
	if root.PersistentFlags().Lookup("yes") == nil {
		root.PersistentFlags().Bool("yes", false, "")
	}
	if root.PersistentFlags().Lookup("dry-run") == nil {
		root.PersistentFlags().Bool("dry-run", false, "")
	}

	liveReg, err := cli.AssembleSchemaRegistry(root)
	if err != nil {
		t.Fatalf("AssembleSchemaRegistry: %v", err)
	}
	effective, err := cli.BuildEffectiveCommandRegistry(root)
	if err != nil {
		t.Fatalf("BuildEffectiveCommandRegistry: %v", err)
	}
	bound, err := cli.BindEffectiveCommandRegistry(root, effective)
	if err != nil {
		t.Fatalf("BindEffectiveCommandRegistry: %v", err)
	}

	liveByCanon := make(map[string]string) // canonical → confirmation
	for _, product := range liveReg.Products {
		for _, tool := range product.Tools {
			liveByCanon[tool.Identity.CanonicalPath] = strings.TrimSpace(tool.Safety.Confirmation)
		}
	}

	type row struct {
		canonical string
		cliPath   string
		gate      string
		detail    string
	}
	var fails []row
	var okRows []row
	checked := 0

	canonOrder := make([]string, 0, len(bound.Commands))
	boundByCanon := make(map[string]cli.BoundCommandSpec, len(bound.Commands))
	for _, cmd := range bound.Commands {
		if cmd.CanonicalPath == "" {
			continue
		}
		canonOrder = append(canonOrder, cmd.CanonicalPath)
		boundByCanon[cmd.CanonicalPath] = cmd
	}
	sort.Strings(canonOrder)

	for _, canonical := range canonOrder {
		cmd := boundByCanon[canonical]
		leaf := cmd.PrimaryCommand
		if leaf == nil {
			fails = append(fails, row{canonical, cmd.PrimaryCLIPath, "", "nil PrimaryCommand"})
			continue
		}
		final, hasFinal := cli.RuntimeContractFinal(leaf)
		if !hasFinal || final.Safety == nil {
			continue
		}
		conf := strings.TrimSpace(final.Safety.Confirmation)
		if conf != "user_required" {
			continue
		}
		checked++
		cliPath := cmd.PrimaryCLIPath

		liveConf, ok := liveByCanon[canonical]
		if !ok {
			fails = append(fails, row{canonical, cliPath, "", "absent from live registry"})
			continue
		}
		if liveConf != "user_required" {
			fails = append(fails, row{canonical, cliPath, "", fmt.Sprintf(
				"ContractFinal=user_required but live ToolSpec.Confirmation=%q", liveConf)})
			continue
		}

		gate := ""
		switch {
		case helpers.HasContractConfirmSafety(leaf) && helpers.HasSheetMutationConfirmationGuard(leaf):
			gate = "declare_leaf+sheet_marker"
		case helpers.HasContractConfirmSafety(leaf):
			gate = "declare_leaf_confirm"
		case helpers.HasSheetMutationConfirmationGuard(leaf):
			gate = "sheet_protect"
		default:
			// NewLeafCommand / Shortcut: NewCommand registers ContractFinal from the
			// same SafetySpec it wires into ConfirmSafety. Prove that SafetySpec is
			// still user_required-operable (closed stdin → confirmation gate).
			leaf.SetIn(strings.NewReader(""))
			leaf.SetOut(io.Discard)
			leaf.SetErr(io.Discard)
			if err := corecmd.ConfirmSafety(leaf, *final.Safety); !isConfirmationGateError(err) {
				fails = append(fails, row{canonical, cliPath, "", fmt.Sprintf(
					"framework leaf: ConfirmSafety(ContractFinal.Safety) = %v, want confirmation gate", err)})
				continue
			}
			// Execute without --yes must not succeed (confirm-first or validate-then-confirm).
			if err := probeConfirmationGate(leaf); err == nil {
				fails = append(fails, row{canonical, cliPath, "",
					"framework leaf: Execute without --yes succeeded"})
				continue
			}
			gate = "framework_confirm_safety"
		}
		okRows = append(okRows, row{canonical, cliPath, gate, "ok"})
	}

	if checked == 0 {
		t.Fatal("no user_required ContractFinal leaves found")
	}
	if len(fails) != 0 {
		for _, f := range fails {
			t.Errorf("%s (%s): %s", f.canonical, f.cliPath, f.detail)
		}
		t.Fatalf("Safety homology failed: %d/%d user_required leaves", len(fails), checked)
	}

	byGate := map[string]int{}
	for _, r := range okRows {
		byGate[r.gate]++
	}
	t.Logf("user_required Safety homology OK: %d leaves gates=%v", checked, byGate)
}

func probeConfirmationGate(leaf *cobra.Command) error {
	root := leaf.Root()
	if root == nil {
		root = leaf
	}
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetIn(strings.NewReader(""))
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	// Rebuild args from CommandPath relative to root.
	path := strings.TrimSpace(strings.TrimPrefix(leaf.CommandPath(), root.CommandPath()))
	fields := strings.Fields(path)
	root.SetArgs(fields)
	return root.Execute()
}

func isConfirmationGateError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "confirmation_required") ||
		strings.Contains(msg, "需要用户确认") ||
		strings.Contains(msg, "用户取消了操作") ||
		strings.Contains(msg, "加 --yes")
}
