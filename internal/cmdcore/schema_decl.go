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

package cmdcore

import (
	"fmt"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
)

// SchemaDecl is the final Schema leaf payload authored on the command
// Contract. When non-empty fields are set, NewCommand embeds them onto the
// Cobra leaf and Schema assembly pass-throughs the values — no reviewed /
// hints parallel authority for those fields.
//
// Shape mirrors cli.ToolSpec groups (minus Parameters/Constraints, which come
// from Flags/Constraints, and minus FieldProvenance, which is derived).
type SchemaDecl struct {
	Title       string
	Description string
	Positionals []PositionalDecl
	Safety      SafetyDecl
	DryRun      *DryRunDecl
	Interface   *InterfaceDecl
	Selection   SelectionDecl
	Identity    IdentityDecl
}

// PositionalDecl is one ordered CLI argument in the Schema positionals list.
type PositionalDecl struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Variadic    bool
	Index       int
}

// SafetyDecl is the final Schema Safety payload. Any field left empty is
// filled from the CommandSpec.Safety tier (or Risk.SafetyDefault() when
// Safety is unset) at embed time, including Idempotency.
type SafetyDecl struct {
	Effect       string
	Risk         string
	Confirmation string
	Idempotency  string
}

// DryRunDecl is a positive --dry-run capability declaration for Schema.
type DryRunDecl struct {
	PreviewKind string
	RemoteReads bool
}

// InterfaceDecl is the final Schema Interface payload (not CLI flags).
type InterfaceDecl struct {
	Mode         string
	Availability string
	Reason       string
	ProductID    string
	RPCName      string
}

// SelectionDecl is the final Schema Selection payload for Agents.
type SelectionDecl struct {
	AgentSummary  string
	UseWhen       []string
	AvoidWhen     []string
	Prerequisites []string
	Tips          []string
	WorkflowRefs  []string
	Examples      []string
}

// IdentityDecl is the final Schema identity payload for a managed leaf.
// Registry may still index paths for discovery; declared values are what Schema delivers.
type IdentityDecl struct {
	ProductID       string
	SourceProductID string
	Name            string
	CLIName         string
	CanonicalPath   string
	CLIPath         string
	PrimaryCLIPath  string
	Group           string
	Aliases         []string
	Source          string
}

// validateSchemaDecl enforces authoring-time homology for declared commands.
// A declared Schema is the sole final source for its fields: downstream
// catalog/Agent gates hard-require description and the reviewed selection
// prose for every effective tool, and declared tools are exempt from
// hint-file coverage — so a declaration missing any of these fields could
// only fail later, opaquely, in generated artifacts. Failing at construction
// keeps the error next to the authoring mistake and prevents silent drift
// between cobra prose (Short/Long/Example) and the published Schema values.
func validateSchemaDecl(spec CommandSpec) {
	if spec.Schema.empty() {
		return
	}
	missing := make([]string, 0, 8)
	if strings.TrimSpace(spec.Schema.Description) == "" {
		missing = append(missing, "Schema.Description")
	}
	if strings.TrimSpace(spec.Schema.Selection.AgentSummary) == "" {
		missing = append(missing, "Schema.Selection.AgentSummary")
	}
	if len(spec.Schema.Selection.UseWhen) == 0 {
		missing = append(missing, "Schema.Selection.UseWhen")
	}
	if len(spec.Schema.Selection.AvoidWhen) == 0 {
		missing = append(missing, "Schema.Selection.AvoidWhen")
	}
	if len(spec.Schema.Selection.Examples) == 0 {
		missing = append(missing, "Schema.Selection.Examples")
	}
	// Interface is an unconditional catalog required key for declared tools
	// (no hints fallback). Safety needs no completeness check: the enum tier
	// (CommandSpec.Safety, falling back to Risk.SafetyDefault()) fills every
	// Safety field including Idempotency, so any declared Schema projects a
	// complete safety block by construction. The flip side of always-on
	// inference: a WRITE command must declare Risk (which also drives runtime
	// confirmation) — an empty Risk publishes the read tier. The detectable
	// half of that mistake (ConfirmFirst without Risk) is caught by
	// validateDispatchDecl.
	if iface := spec.Schema.Interface; iface == nil ||
		strings.TrimSpace(iface.Mode) == "" || strings.TrimSpace(iface.Availability) == "" {
		missing = append(missing, "Schema.Interface (mode/availability)")
	} else if (iface.Mode == cli.InterfaceModeComposite || iface.Availability == cli.InterfaceUnavailable) &&
		strings.TrimSpace(iface.Reason) == "" {
		missing = append(missing, "Schema.Interface.Reason")
	}
	if len(missing) > 0 {
		panic(fmt.Sprintf(
			"command %q declares Schema but is missing %s: a declared Schema is the final source and must carry the full reviewed prose (no hints fallback)",
			spec.Use, strings.Join(missing, ", ")))
	}
}

// empty reports whether no SchemaDecl field was authored.
func (s SchemaDecl) empty() bool {
	if strings.TrimSpace(s.Title) != "" || strings.TrimSpace(s.Description) != "" {
		return false
	}
	if len(s.Positionals) > 0 {
		return false
	}
	if strings.TrimSpace(s.Safety.Effect) != "" || strings.TrimSpace(s.Safety.Risk) != "" ||
		strings.TrimSpace(s.Safety.Confirmation) != "" || strings.TrimSpace(s.Safety.Idempotency) != "" {
		return false
	}
	if s.DryRun != nil && strings.TrimSpace(s.DryRun.PreviewKind) != "" {
		return false
	}
	if s.Interface != nil && (strings.TrimSpace(s.Interface.Mode) != "" || strings.TrimSpace(s.Interface.ProductID) != "" ||
		strings.TrimSpace(s.Interface.RPCName) != "" || strings.TrimSpace(s.Interface.Availability) != "" ||
		strings.TrimSpace(s.Interface.Reason) != "") {
		return false
	}
	if strings.TrimSpace(s.Selection.AgentSummary) != "" || len(s.Selection.UseWhen) > 0 ||
		len(s.Selection.AvoidWhen) > 0 || len(s.Selection.Examples) > 0 ||
		len(s.Selection.Prerequisites) > 0 || len(s.Selection.Tips) > 0 ||
		len(s.Selection.WorkflowRefs) > 0 {
		return false
	}
	if strings.TrimSpace(s.Identity.ProductID) != "" || strings.TrimSpace(s.Identity.Name) != "" ||
		strings.TrimSpace(s.Identity.CanonicalPath) != "" || strings.TrimSpace(s.Identity.CLIPath) != "" ||
		strings.TrimSpace(s.Identity.PrimaryCLIPath) != "" || len(s.Identity.Aliases) > 0 {
		return false
	}
	return true
}
