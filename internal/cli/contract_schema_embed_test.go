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

package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCrossPlatformCoverageAnnotateRuntimeRiskEmbedsContractMarker(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	AnnotateRuntimeRisk(cmd, "write")
	got, ok := RuntimeContractRisk(cmd)
	if !ok || got != "write" {
		t.Fatalf("RuntimeContractRisk = %q %v", got, ok)
	}
	if cmd.Annotations[runtimeSchemaContractAnnotation] != "command" {
		t.Fatalf("contract marker = %q", cmd.Annotations[runtimeSchemaContractAnnotation])
	}
	AnnotateRuntimeRisk(cmd, "")
	if got, _ = RuntimeContractRisk(cmd); got != "write" {
		t.Fatalf("empty AnnotateRuntimeRisk must be no-op, got %q", got)
	}
}

func TestCrossPlatformCoverageApplyContractRiskToSafety(t *testing.T) {
	base := SafetySpec{Idempotency: "unknown", Risk: "low", Confirmation: "not_required", Effect: "read"}
	got := applyContractRiskToSafety(base, "high-risk-write")
	if got.Effect != "destructive" || got.Risk != "high" || got.Confirmation != "user_required" {
		t.Fatalf("high-risk overlay = %#v", got)
	}
	if got.Idempotency != "unknown" {
		t.Fatalf("idempotency should be preserved, got %q", got.Idempotency)
	}
	got = applyContractRiskToSafety(base, "write")
	if got.Effect != "write" || got.Risk != "medium" || got.Confirmation != "user_required" {
		t.Fatalf("write overlay = %#v", got)
	}
	got = applyContractRiskToSafety(base, "read")
	if got.Effect != "read" || got.Risk != "low" || got.Confirmation != "not_required" {
		t.Fatalf("read overlay = %#v", got)
	}
}

func TestCrossPlatformCoverageAnnotateRuntimeGateDeclareOrAnnotate(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	if HasDeclaredOrAnnotatedConfirmation(cmd) {
		t.Fatal("bare command must not claim confirmation coverage")
	}
	AnnotateRuntimeGate(cmd, "devAppRequireWriteGuard")
	got, ok := RuntimeContractGate(cmd)
	if !ok || got != "devAppRequireWriteGuard" {
		t.Fatalf("RuntimeContractGate = %q %v", got, ok)
	}
	if !HasDeclaredOrAnnotatedConfirmation(cmd) {
		t.Fatal("annotated gate must satisfy declare-OR-annotate")
	}
	if cmd.Annotations[runtimeSchemaContractAnnotation] != "command" {
		t.Fatalf("contract marker = %q", cmd.Annotations[runtimeSchemaContractAnnotation])
	}
}

func TestCrossPlatformCoverageApplyContractGateToSafety(t *testing.T) {
	base := SafetySpec{Confirmation: "not_required", Effect: "read", Risk: "low"}
	got := applyContractGateToSafety(base, "devAppRequireWriteGuard")
	if got.Confirmation != "user_required" {
		t.Fatalf("gate must force user_required, got %#v", got)
	}
	if got.Effect != "write" || got.EffectSource != "corecmd.contract_gate" {
		t.Fatalf("gate effect overlay = %#v", got)
	}
	if got.Risk != "medium" {
		t.Fatalf("gate risk overlay = %#v", got)
	}
	reviewed := SafetySpec{Confirmation: "not_required", Effect: "destructive", Risk: "high", EffectSource: "reviewed"}
	got = applyContractGateToSafety(reviewed, "devAppRequireWriteGuard")
	if got.Effect != "destructive" || got.Risk != "high" || got.Confirmation != "user_required" {
		t.Fatalf("gate must keep reviewed effect/risk: %#v", got)
	}
	if got = applyContractGateToSafety(base, "   "); got != base {
		t.Fatalf("blank gate must be a no-op, got %#v", got)
	}
}

func TestCrossPlatformCoverageRuntimeContractAnnotationNilAndBlankGuards(t *testing.T) {
	// Nil-command annotators must be safe no-ops.
	AnnotateRuntimeFlagDescription(nil, "flag", "description")
	AnnotateRuntimeContract(nil)
	AnnotateRuntimeRisk(nil, "write")
	AnnotateRuntimeGate(nil, "devAppRequireWriteGuard")

	cmd := &cobra.Command{Use: "x"}
	AnnotateRuntimeGate(cmd, "   ")
	if _, ok := RuntimeContractGate(cmd); ok {
		t.Fatal("blank AnnotateRuntimeGate must not record a gate")
	}
	if cmd.Annotations != nil && cmd.Annotations[runtimeSchemaContractAnnotation] == "command" {
		t.Fatal("blank AnnotateRuntimeGate must not mark the command as Contract")
	}

	// A present-but-blank gate annotation must read back as no gate.
	blank := &cobra.Command{Use: "y", Annotations: map[string]string{
		runtimeSchemaRuntimeGateAnnotation: "   ",
	}}
	if _, ok := RuntimeContractGate(blank); ok {
		t.Fatal("blank runtime_gate annotation must not report a gate")
	}
}

func TestCrossPlatformCoverageHasDeclaredOrAnnotatedConfirmationDeclaredAndRiskBranches(t *testing.T) {
	declared := &cobra.Command{Use: "declared"}
	t.Cleanup(func() { ClearRuntimeContractFinalForTest(declared) })
	RegisterRuntimeContractFinal(declared, ContractFinalPayload{
		Safety: &SafetySpec{Confirmation: "not_required"},
	})
	if !HasDeclaredOrAnnotatedConfirmation(declared) {
		t.Fatal("typed Contract SafetySpec confirmation must satisfy declare-OR-annotate")
	}

	risky := &cobra.Command{Use: "risky"}
	AnnotateRuntimeRisk(risky, "read")
	if !HasDeclaredOrAnnotatedConfirmation(risky) {
		t.Fatal("Contract Risk annotation must satisfy declare-OR-annotate")
	}
}
