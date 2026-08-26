// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/spf13/cobra"
)

func TestCrossPlatformCoverageRenderSelectionGuidanceUsesResolvedFields(t *testing.T) {
	cmd := &cobra.Command{Use: "leaf"}
	var out bytes.Buffer
	cmd.SetOut(&out)
	renderSelectionGuidance(cmd, CommandSelection{
		UseWhen:       []string{" use reviewed route "},
		AvoidWhen:     []string{"avoid a different product"},
		Prerequisites: []string{"resolve the target ID"},
		Tips:          []string{"prefer structured output"},
	})
	rendered := out.String()
	for _, want := range []string{
		"When to use:\n  - use reviewed route",
		"Avoid when:\n  - avoid a different product",
		"Prerequisites:\n  - resolve the target ID",
		"Tips:\n  - prefer structured output",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered guidance missing %q: %q", want, rendered)
		}
	}
}

func TestCrossPlatformCoverageRenderHelpReferences(t *testing.T) {
	cmd := &cobra.Command{Use: "service"}
	var out bytes.Buffer
	cmd.SetOut(&out)
	renderHelpReferences(cmd, contract.HelpReferences{
		RelatedSkills: []string{"dingtalk-chat"},
		Documentation: []contract.HelpDocumentation{{Label: "Chat guide", URL: "https://example.com/chat"}},
	})
	rendered := out.String()
	for _, want := range []string{"Related skills:\n  - dingtalk-chat", "Documentation:\n  - Chat guide: https://example.com/chat"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered references missing %q: %q", want, rendered)
		}
	}
}
