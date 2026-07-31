// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// schemaHintDecl is one command's Safety + Schema migrated from reviewed
// schema_hints selection/metadata into compiled Go.
type schemaHintDecl struct {
	Safety      SafetySpec
	Description string
	DryRun      *DryRunSpec
	Interface   *InterfaceSpec
	Selection   SelectionSpec
}

// lookupSchemaHintDecl returns the migrated declaration for a canonical path.
func lookupSchemaHintDecl(canonical string) (schemaHintDecl, bool) {
	canonical = strings.TrimSpace(canonical)
	if canonical == "" {
		return schemaHintDecl{}, false
	}
	d, ok := schemaHintDeclsByCanonical[canonical]
	return d, ok
}

// attachSchemaHintDecl registers ContractFinal from a migrated hint declaration
// without replacing RunE/Execute. Completeness is enforced here so bind-time
// attaches cannot ship a partial overlay.
func attachSchemaHintDecl(cmd *cobra.Command, decl schemaHintDecl) {
	if cmd == nil {
		return
	}
	if strings.TrimSpace(decl.Description) == "" {
		panic(fmt.Sprintf("schema hint decl for %s missing Description", cmd.CommandPath()))
	}
	if strings.TrimSpace(decl.Selection.AgentSummary) == "" ||
		len(decl.Selection.UseWhen) == 0 || len(decl.Selection.AvoidWhen) == 0 ||
		len(decl.Selection.Examples) == 0 {
		panic(fmt.Sprintf("schema hint decl for %s missing Selection prose", cmd.CommandPath()))
	}
	if decl.Interface == nil || strings.TrimSpace(decl.Interface.Mode) == "" ||
		strings.TrimSpace(decl.Interface.Availability) == "" {
		panic(fmt.Sprintf("schema hint decl for %s missing Interface", cmd.CommandPath()))
	}
	if (decl.Interface.Mode == InterfaceModeComposite ||
		decl.Interface.Availability == InterfaceUnavailable) &&
		strings.TrimSpace(decl.Interface.Reason) == "" {
		panic(fmt.Sprintf("schema hint decl for %s missing Interface.Reason", cmd.CommandPath()))
	}

	safety := decl.Safety
	if strings.TrimSpace(safety.Effect) == "" && strings.TrimSpace(safety.Risk) == "" &&
		strings.TrimSpace(safety.Confirmation) == "" && strings.TrimSpace(safety.Idempotency) == "" {
		safety = SafetySpec{
			Effect: "read", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		}
	}
	safety.EffectSource = "corecmd.contract"

	payload := ContractFinalPayload{
		Title:       firstNonEmptyTrim(cmd.Short),
		Description: firstNonEmptyTrim(decl.Description, cmd.Long),
		Safety:      &safety,
		DryRun:      decl.DryRun,
		Interface:   decl.Interface,
		Selection: &SelectionSpec{
			AgentSummary:  strings.TrimSpace(decl.Selection.AgentSummary),
			UseWhen:       decl.Selection.UseWhen,
			AvoidWhen:     decl.Selection.AvoidWhen,
			Prerequisites: decl.Selection.Prerequisites,
			Tips:          decl.Selection.Tips,
			WorkflowRefs:  decl.Selection.WorkflowRefs,
			Examples:      decl.Selection.Examples,
		},
	}
	RegisterRuntimeContractFinal(cmd, payload)
}

func firstNonEmptyTrim(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
