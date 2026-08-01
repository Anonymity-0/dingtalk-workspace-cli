// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package homology

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/app"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/contractfinal"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

// TestAllCommandsContractFinalConsistentWithLiveAndEmbeddedCatalog walks every
// reviewed registry leaf and checks, one by one:
//  1. PrimaryCommand has runtime ContractFinal (command-side declaration)
//  2. Live assembly ToolSpec fields match that ContractFinal
//  3. Embedded full-leaf shard fields match live ToolSpec (incl. examples)
func TestAllCommandsContractFinalConsistentWithLiveAndEmbeddedCatalog(t *testing.T) {
	root := app.NewSchemaSourceRootCommand()
	liveReg, err := cli.AssembleSchemaRegistry(root)
	if err != nil {
		t.Fatalf("AssembleSchemaRegistry(live) error = %v", err)
	}
	embedByCanon, err := loadEmbeddedFullLeafTools()
	if err != nil {
		t.Fatalf("load embedded full leaf shards: %v", err)
	}

	effective, err := cli.BuildEffectiveCommandRegistry(root)
	if err != nil {
		t.Fatalf("BuildEffectiveCommandRegistry() error = %v", err)
	}
	bound, err := cli.BindEffectiveCommandRegistry(root, effective)
	if err != nil {
		t.Fatalf("BindEffectiveCommandRegistry() error = %v", err)
	}

	liveByCanon := indexLiveTools(liveReg)

	type row struct {
		canonical string
		cliPath   string
		status    string
		detail    string
	}
	rows := make([]row, 0, len(bound.Commands))
	var failCount int
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
		cliPath := cmd.PrimaryCLIPath
		liveTool, ok := liveByCanon[canonical]
		if !ok {
			failCount++
			rows = append(rows, row{canonical, cliPath, "FAIL", "absent from live registry"})
			continue
		}
		embedTool, ok := embedByCanon[canonical]
		if !ok {
			failCount++
			rows = append(rows, row{canonical, cliPath, "FAIL", "absent from embedded full leaf shard"})
			continue
		}
		final, has := contractfinal.RuntimeContractFinal(cmd.PrimaryCommand)
		if !has {
			failCount++
			rows = append(rows, row{canonical, cliPath, "FAIL", "no contract.RuntimeContractFinal on PrimaryCommand"})
			continue
		}
		checked++
		problems := compareFinalToLive(canonical, final, liveTool)
		problems = append(problems, compareLiveToEmbedded(canonical, liveTool, embedTool)...)
		if len(problems) > 0 {
			failCount++
			rows = append(rows, row{canonical, cliPath, "FAIL", strings.Join(problems, "; ")})
			continue
		}
		if liveTool.MetadataSource != "corecmd.contract" {
			failCount++
			rows = append(rows, row{canonical, cliPath, "FAIL", "metadata_source=" + liveTool.MetadataSource})
			continue
		}
		rows = append(rows, row{canonical, cliPath, "OK", "ContractFinal=live=catalog"})
	}

	reportPath := filepath.Join(t.TempDir(), "contract_final_per_command.txt")
	var b strings.Builder
	for i, r := range rows {
		line := fmt.Sprintf("%04d %s %-48s %-60s %s\n", i+1, r.status, r.canonical, r.cliPath, r.detail)
		b.WriteString(line)
		if r.status != "OK" {
			t.Errorf("%s", strings.TrimSpace(line))
		}
	}
	summary := fmt.Sprintf("SUMMARY checked=%d fail=%d ok=%d live=%d embed=%d\n",
		checked, failCount, checked-failCount, len(liveByCanon), len(embedByCanon))
	b.WriteString(summary)
	if err := os.WriteFile(reportPath, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	t.Logf("per-command report: %s", reportPath)
	t.Logf("%s", strings.TrimSpace(summary))
	// Print every line so -v shows 逐个检查.
	for _, r := range rows {
		t.Logf("%s %-48s %s", r.status, r.canonical, r.cliPath)
	}

	if failCount > 0 {
		t.Fatalf("per-command consistency failed: %d/%d", failCount, len(rows))
	}
	if checked != 845 {
		t.Fatalf("checked = %d, want 845 reviewed tools", checked)
	}
	if err := cli.ValidateEmbeddedRuntimeSchemaCompleteness(app.NewSchemaSourceRootCommand()); err != nil {
		t.Fatalf("ValidateEmbeddedRuntimeSchemaCompleteness() error = %v", err)
	}
}

type liveToolView struct {
	Description     string
	MetadataSource  string
	Effect          string
	Risk            string
	Confirmation    string
	Idempotency     string
	InterfaceMode   string
	Availability    string
	InterfacePID    string
	InterfaceRPC    string
	InterfaceReason string
	AgentSummary    string
	UseWhen         []string
	AvoidWhen       []string
	Examples        []string
}

type embedToolView struct {
	Description    string
	MetadataSource string
	Effect         string
	Risk           string
	Confirmation   string
	Idempotency    string
	InterfaceMode  string
	Availability   string
	InterfacePID   string
	InterfaceRPC   string
	AgentSummary   string
	UseWhen        []string
	AvoidWhen      []string
	Examples       []string
}

func indexLiveTools(reg cli.SchemaRegistry) map[string]liveToolView {
	out := make(map[string]liveToolView)
	for _, product := range reg.Products {
		for _, tool := range product.Tools {
			view := liveToolView{
				Description:     tool.Description,
				MetadataSource:  tool.MetadataSource,
				Effect:          tool.Safety.Effect,
				Risk:            tool.Safety.Risk,
				Confirmation:    tool.Safety.Confirmation,
				Idempotency:     tool.Safety.Idempotency,
				InterfaceMode:   tool.Interface.Mode,
				Availability:    tool.Interface.Availability,
				InterfaceReason: tool.Interface.Reason,
				AgentSummary:    tool.Selection.AgentSummary,
				UseWhen:         append([]string(nil), tool.Selection.UseWhen...),
				AvoidWhen:       append([]string(nil), tool.Selection.AvoidWhen...),
				Examples:        append([]string(nil), tool.Selection.Examples...),
			}
			if tool.Interface.Ref != nil {
				view.InterfacePID = tool.Interface.Ref.ProductID
				view.InterfaceRPC = tool.Interface.Ref.RPCName
			}
			out[tool.Identity.CanonicalPath] = view
		}
	}
	return out
}

func repoCLIDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	// This file lives in internal/cli/homology; schema_catalog is under internal/cli.
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

func loadEmbeddedFullLeafTools() (map[string]embedToolView, error) {
	toolsDir := filepath.Join(repoCLIDir(), "schema_catalog", "tools")
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]embedToolView)
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(toolsDir, ent.Name()))
		if err != nil {
			return nil, err
		}
		var shard struct {
			Tools map[string]map[string]any `json:"tools"`
		}
		if err := json.Unmarshal(data, &shard); err != nil {
			return nil, fmt.Errorf("%s: %w", ent.Name(), err)
		}
		for key, raw := range shard.Tools {
			canonical := asString(raw["canonical_path"])
			if canonical == "" {
				canonical = key
			}
			view := embedToolView{
				Description:    asString(raw["description"]),
				MetadataSource: asString(raw["metadata_source"]),
				Effect:         asString(raw["effect"]),
				Risk:           asString(raw["risk"]),
				Confirmation:   asString(raw["confirmation"]),
				Idempotency:    asString(raw["idempotency"]),
				InterfaceMode:  asString(raw["interface_mode"]),
				Availability:   asString(raw["availability"]),
				AgentSummary:   asString(raw["agent_summary"]),
				UseWhen:        asStringSlice(raw["use_when"]),
				AvoidWhen:      asStringSlice(raw["avoid_when"]),
				Examples:       asStringSlice(raw["examples"]),
			}
			if ref, ok := raw["interface_ref"].(map[string]any); ok {
				view.InterfacePID = asString(ref["product_id"])
				view.InterfaceRPC = asString(ref["rpc_name"])
			}
			out[canonical] = view
		}
	}
	return out, nil
}

func compareFinalToLive(canonical string, final contract.ContractFinalPayload, tool liveToolView) []string {
	var problems []string
	add := func(field, want, got string) {
		if strings.TrimSpace(want) == "" {
			return
		}
		if strings.TrimSpace(want) != strings.TrimSpace(got) {
			problems = append(problems, fmt.Sprintf("%s: final=%q live=%q", field, want, got))
		}
	}
	// Description is intentionally not compared: ContractFinal stores the
	// authored ContractDecl.Description, while Catalog delivery prefers Cobra
	// Long when present (provenance cobra_help). live↔embed still checks the
	// delivered description string.
	_ = final.Description
	_ = tool.Description
	if final.Safety != nil {
		add("effect", final.Safety.Effect, tool.Effect)
		add("risk", final.Safety.Risk, tool.Risk)
		add("confirmation", final.Safety.Confirmation, tool.Confirmation)
		add("idempotency", final.Safety.Idempotency, tool.Idempotency)
	}
	if final.Interface != nil {
		add("interface_mode", final.Interface.Mode, tool.InterfaceMode)
		add("availability", final.Interface.Availability, tool.Availability)
		add("interface_reason", final.Interface.Reason, tool.InterfaceReason)
		if final.Interface.Ref != nil {
			add("interface_product", final.Interface.Ref.ProductID, tool.InterfacePID)
			add("interface_rpc", final.Interface.Ref.RPCName, tool.InterfaceRPC)
		}
	}
	if final.Selection != nil {
		add("agent_summary", final.Selection.AgentSummary, tool.AgentSummary)
		if !reflect.DeepEqual(final.Selection.UseWhen, tool.UseWhen) {
			problems = append(problems, "use_when mismatch")
		}
		if !reflect.DeepEqual(final.Selection.AvoidWhen, tool.AvoidWhen) {
			problems = append(problems, "avoid_when mismatch")
		}
		if !reflect.DeepEqual(final.Selection.Examples, tool.Examples) {
			problems = append(problems, "examples mismatch")
		}
	}
	_ = canonical
	return problems
}

func compareLiveToEmbedded(canonical string, live liveToolView, embed embedToolView) []string {
	var problems []string
	add := func(field, a, b string) {
		if strings.TrimSpace(a) != strings.TrimSpace(b) {
			problems = append(problems, fmt.Sprintf("%s: live=%q embed=%q", field, a, b))
		}
	}
	add("description", live.Description, embed.Description)
	add("effect", live.Effect, embed.Effect)
	add("risk", live.Risk, embed.Risk)
	add("confirmation", live.Confirmation, embed.Confirmation)
	add("idempotency", live.Idempotency, embed.Idempotency)
	add("interface_mode", live.InterfaceMode, embed.InterfaceMode)
	add("availability", live.Availability, embed.Availability)
	add("agent_summary", live.AgentSummary, embed.AgentSummary)
	add("metadata_source", live.MetadataSource, embed.MetadataSource)
	add("interface_product", live.InterfacePID, embed.InterfacePID)
	add("interface_rpc", live.InterfaceRPC, embed.InterfaceRPC)
	if !reflect.DeepEqual(live.UseWhen, embed.UseWhen) {
		problems = append(problems, "use_when live!=embed")
	}
	if !reflect.DeepEqual(live.AvoidWhen, embed.AvoidWhen) {
		problems = append(problems, "avoid_when live!=embed")
	}
	if !reflect.DeepEqual(live.Examples, embed.Examples) {
		problems = append(problems, "examples live!=embed")
	}
	_ = canonical
	return problems
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asStringSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
