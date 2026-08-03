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

package app

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
)

// TestIdentityCollectorProbe is the Phase-1 identity-deregistry probe. It is
// opt-in so it does not gate normal test runs until byte-equivalence between
// collected (Contract.Identity) and reviewed (schema_command_registry)
// identity is confirmed. Run with:
//
//	DWS_IDENTITY_PROBE=1 go test -count=1 -run TestIdentityCollectorProbe ./internal/app
//
// Exit criterion: SourceHash of the collected EffectiveCommandRegistry equals
// the reviewed one, the field-diff list is empty, and no leaf is missing an
// Identity without a matching reviewed exclusion.
func TestIdentityCollectorProbe(t *testing.T) {
	if os.Getenv("DWS_IDENTITY_PROBE") == "" {
		t.Skip("identity-deregistry probe is opt-in: set DWS_IDENTITY_PROBE=1")
	}

	root := NewRootCommand()
	collected, report, err := cli.CollectIdentitySpecs(root)
	if err != nil {
		t.Fatalf("collect identity specs: %v", err)
	}
	reviewed, err := cli.ReviewedCommandSpecs()
	if err != nil {
		t.Fatalf("load reviewed command specs: %v", err)
	}

	t.Logf("walk leaves=%d withIdentity=%d hiddenPrimaries=%d excluded=%d noIdentity=%d collected=%d reviewed=%d",
		report.Leaves, report.WithIdentity, report.HiddenPrimaries, report.Excluded, len(report.NoIdentity), len(collected), len(reviewed))

	collectedEffective, err := cli.BuildEffectiveFromSpecs(collected)
	if err != nil {
		t.Fatalf("build collected effective registry: %v", err)
	}
	reviewedEffective, err := cli.BuildEffectiveFromSpecs(reviewed)
	if err != nil {
		t.Fatalf("build reviewed effective registry: %v", err)
	}
	collectedHash := collectedEffective.SourceHash()
	reviewedHash := reviewedEffective.SourceHash()
	t.Logf("collected SourceHash=%s", collectedHash)
	t.Logf("reviewed  SourceHash=%s", reviewedHash)

	collectedCanonicals := make(map[string]bool, len(collectedEffective.Commands))
	for _, spec := range collectedEffective.Commands {
		collectedCanonicals[strings.TrimSpace(spec.CanonicalPath)] = true
	}
	var missingPrimaries []cli.CommandSpec
	for _, spec := range reviewedEffective.Commands {
		if !collectedCanonicals[strings.TrimSpace(spec.CanonicalPath)] {
			missingPrimaries = append(missingPrimaries, spec)
		}
	}
	report.MissingPrimary = make([]string, 0, len(missingPrimaries))
	for _, spec := range missingPrimaries {
		report.MissingPrimary = append(report.MissingPrimary, spec.CanonicalPath)
	}
	sort.Strings(report.MissingPrimary)
	for _, canonical := range report.MissingPrimary {
		t.Logf("MISSING_PRIMARY %s", canonical)
	}
	for _, diagnostic := range cli.DiagnoseMissingPrimaries(root, missingPrimaries) {
		t.Logf("DIAG %s", diagnostic)
	}
	for _, leaf := range report.NoIdentity {
		t.Logf("NO_IDENTITY_LEAF %s", leaf)
	}

	problems := cli.CompareCommandSpecEquivalence(collectedEffective.Commands, reviewedEffective.Commands)
	for _, problem := range problems {
		t.Logf("DIFF %s", problem)
	}

	if collectedHash != reviewedHash || len(problems) > 0 || len(missingPrimaries) > 0 {
		t.Fatalf("identity collection not byte-equivalent to reviewed registry: field_diffs=%d missing_primary=%d hash_match=%v",
			len(problems), len(missingPrimaries), collectedHash == reviewedHash)
	}
}
