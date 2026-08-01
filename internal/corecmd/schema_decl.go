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

package corecmd

import (
	"fmt"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

// SchemaDecl is the final Schema leaf payload authored on the command
// Contract. When non-empty fields are set, NewCommand embeds them onto the
// Cobra leaf and Schema assembly pass-throughs the values — no reviewed /
// hints parallel authority for those fields.
//
// Shape mirrors cli.ToolSpec groups (minus Constraints, which come from
// Constraints, and minus FieldProvenance, which is derived).
type SchemaDecl struct {
	Title       string
	Description string
	Positionals []PositionalDecl
	Parameters  []ParamDecl
	DryRun      *DryRunDecl
	Interface   *InterfaceDecl
	Selection   SelectionDecl
	Identity    IdentityDecl
}

// ParamDecl declares Schema facts about a flag that the command registers
// itself. It exists because metadata-mode commands (DeclareLeafMetadata) own
// their flag registration and cannot use FlagSpec; without this channel their
// parameter facts could only live in schema_hints/metadata overlays.
//
// Each field maps to a dws.schema.* annotation at rank native_annotation (620),
// which outranks tool_schema_hint (500). A declared value therefore wins over
// any remaining hint overlay for the same flag, making the overlay redundant
// once the declaration is in place.
type ParamDecl struct {
	// Name is the cobra flag name (kebab-case), e.g. "record-ids".
	Name string
	// Property is the interface property name (camelCase), e.g. "recordIds".
	// Empty means the flag name is used as-is.
	Property string
	// Required overrides the cobra-level required marker for Schema purposes.
	// Use a pointer so "not declared" (nil) is distinct from "explicitly false".
	Required *bool
	// InterfaceType is the wire type for the interface property, e.g. "string",
	// "integer", "boolean", "array". Empty means derived from the flag kind.
	InterfaceType string
	// Description overrides the flag's help text for Schema purposes.
	Description string
	// RequiredWhen is a conditional-required expression, e.g. "identity=bot".
	RequiredWhen string
	// Enum restricts accepted values (string flags only).
	Enum []string
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
func validateSchemaDecl(spec Spec) {
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
	// (no hints fallback). Spec.Safety is validated separately and is
	// the single source for both runtime confirmation and the Schema block.
	if iface := spec.Schema.Interface; iface == nil ||
		strings.TrimSpace(iface.Mode) == "" || strings.TrimSpace(iface.Availability) == "" {
		missing = append(missing, "Schema.Interface (mode/availability)")
	} else if (iface.Mode == contract.InterfaceModeComposite || iface.Availability == contract.InterfaceUnavailable) &&
		strings.TrimSpace(iface.Reason) == "" {
		missing = append(missing, "Schema.Interface.Reason")
	}
	if len(missing) > 0 {
		panic(fmt.Sprintf(
			"command %q declares Schema but is missing %s: a declared Schema is the final source and must carry the full reviewed prose (no hints fallback)",
			spec.Use, strings.Join(missing, ", ")))
	}
}

// Empty reports whether no SchemaDecl field was authored.
func (s SchemaDecl) Empty() bool {
	return s.empty()
}

// empty reports whether no SchemaDecl field was authored.
func (s SchemaDecl) empty() bool {
	if strings.TrimSpace(s.Title) != "" || strings.TrimSpace(s.Description) != "" {
		return false
	}
	if len(s.Positionals) > 0 {
		return false
	}
	if len(s.Parameters) > 0 {
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
