// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestCrossPlatformCoverageChatCatalogSafetyMetadataIsExplicit(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	root.AddCommand(newChatCommand())

	checked := 0
	writesChecked := 0
	walkChatCatalogLeaves(root, func(cmd *cobra.Command) {
		final, ok := contractfinal.RuntimeContractFinal(cmd)
		if !ok || final.Identity == nil || final.Safety == nil {
			return
		}
		if strings.TrimSpace(final.Identity.ProductID) != "chat" {
			return
		}
		checked++
		canonical := strings.TrimSpace(final.Identity.CanonicalPath)
		safety := final.Safety
		if safety.Effect == "" || safety.Risk == "" ||
			safety.Confirmation == "" || safety.Idempotency == "" {
			t.Fatalf("chat Catalog leaf %s has incomplete Safety: %+v", canonical, *safety)
		}
		if safety.Effect == "read" {
			if safety.Risk != "low" ||
				safety.Confirmation != "not_required" ||
				safety.Idempotency != "idempotent" {
				t.Fatalf("chat read Catalog leaf %s Safety = %+v, want read/low/not_required/idempotent",
					canonical, *safety)
			}
		}
		cliPath := strings.TrimSpace(final.Identity.PrimaryCLIPath)
		if cliPath == "" {
			cliPath = strings.TrimSpace(final.Identity.CLIPath)
		}
		if strings.HasPrefix(cliPath, "chat +") || (safety.Effect != "write" && safety.Effect != "destructive") {
			return
		}
		writesChecked++
		if safety.Idempotency == "unknown" {
			t.Fatalf("formal chat write %s must declare retry safety, got idempotency=unknown", canonical)
		}
		if hasChatDeduplicationParameter(cmd, final.Parameters) && safety.Idempotency != "idempotent" {
			t.Fatalf("formal chat write %s exposes a deduplication parameter but idempotency=%s, want idempotent", canonical, safety.Idempotency)
		}
	})
	if checked == 0 {
		t.Fatal("no chat Catalog leaves checked")
	}
	if writesChecked == 0 {
		t.Fatal("no formal chat write Catalog leaves checked")
	}
}

func hasChatDeduplicationParameter(cmd *cobra.Command, parameters []contract.ParamDecl) bool {
	isDeduplicationName := func(name string) bool {
		normalized := strings.NewReplacer("-", "", "_", "").Replace(strings.ToLower(strings.TrimSpace(name)))
		switch normalized {
		case "uuid", "clienttoken", "idempotencykey":
			return true
		default:
			return false
		}
	}
	for _, parameter := range parameters {
		if isDeduplicationName(parameter.Name) || isDeduplicationName(parameter.Property) {
			return true
		}
	}
	found := false
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if isDeduplicationName(flag.Name) {
			found = true
		}
	})
	return found
}

func walkChatCatalogLeaves(cmd *cobra.Command, fn func(*cobra.Command)) {
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
		walkChatCatalogLeaves(child, fn)
	}
}
