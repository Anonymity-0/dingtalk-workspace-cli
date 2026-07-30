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

import "strings"

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

// SafetyDecl is the final Schema Safety payload. Empty Effect/Risk/Confirmation
// may be filled from CommandSpec.Risk at embed time; Idempotency is declaration-only.
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
