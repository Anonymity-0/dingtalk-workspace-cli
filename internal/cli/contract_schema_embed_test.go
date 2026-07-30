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

func TestAnnotateRuntimeRiskEmbedsContractMarker(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	AnnotateRuntimeRisk(cmd, "write")
	got, ok := RuntimeContractRisk(cmd)
	if !ok || got != "write" {
		t.Fatalf("RuntimeContractRisk = %q %v", got, ok)
	}
	if cmd.Annotations[runtimeSchemaContractAnnotation] != "cmdcore" {
		t.Fatalf("contract marker = %q", cmd.Annotations[runtimeSchemaContractAnnotation])
	}
	AnnotateRuntimeRisk(cmd, "")
	if got, _ = RuntimeContractRisk(cmd); got != "write" {
		t.Fatalf("empty AnnotateRuntimeRisk must be no-op, got %q", got)
	}
}

func TestApplyContractRiskToSafety(t *testing.T) {
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

func TestAnnotateRuntimeGateDeclareOrAnnotate(t *testing.T) {
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
	if cmd.Annotations[runtimeSchemaContractAnnotation] != "cmdcore" {
		t.Fatalf("contract marker = %q", cmd.Annotations[runtimeSchemaContractAnnotation])
	}
}

func TestApplyContractGateToSafety(t *testing.T) {
	base := SafetySpec{Confirmation: "not_required", Effect: "read", Risk: "low"}
	got := applyContractGateToSafety(base, "devAppRequireWriteGuard")
	if got.Confirmation != "user_required" {
		t.Fatalf("gate must force user_required, got %#v", got)
	}
	if got.Effect != "write" || got.EffectSource != "cmdcore.contract_gate" {
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
}
