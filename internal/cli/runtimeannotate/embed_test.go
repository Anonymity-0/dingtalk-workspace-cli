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

package runtimeannotate

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
	if cmd.Annotations[AnnotationContract] != "command" {
		t.Fatalf("contract marker = %q", cmd.Annotations[AnnotationContract])
	}
	AnnotateRuntimeRisk(cmd, "")
	if got, _ = RuntimeContractRisk(cmd); got != "write" {
		t.Fatalf("empty AnnotateRuntimeRisk must be no-op, got %q", got)
	}
}

func TestCrossPlatformCoverageAnnotateRuntimeGate(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	AnnotateRuntimeGate(cmd, "devAppRequireWriteGuard")
	got, ok := RuntimeContractGate(cmd)
	if !ok || got != "devAppRequireWriteGuard" {
		t.Fatalf("RuntimeContractGate = %q %v", got, ok)
	}
	if cmd.Annotations[AnnotationContract] != "command" {
		t.Fatalf("contract marker = %q", cmd.Annotations[AnnotationContract])
	}
}

func TestCrossPlatformCoverageRuntimeContractAnnotationNilAndBlankGuards(t *testing.T) {
	AnnotateRuntimeFlagDescription(nil, "flag", "description")
	AnnotateRuntimeContract(nil)
	AnnotateRuntimeRisk(nil, "write")
	AnnotateRuntimeGate(nil, "devAppRequireWriteGuard")

	cmd := &cobra.Command{Use: "x"}
	AnnotateRuntimeGate(cmd, "   ")
	if _, ok := RuntimeContractGate(cmd); ok {
		t.Fatal("blank AnnotateRuntimeGate must not record a gate")
	}
	if cmd.Annotations != nil && cmd.Annotations[AnnotationContract] == "command" {
		t.Fatal("blank AnnotateRuntimeGate must not mark the command as Contract")
	}

	blank := &cobra.Command{Use: "y", Annotations: map[string]string{
		AnnotationRuntimeGate: "   ",
	}}
	if _, ok := RuntimeContractGate(blank); ok {
		t.Fatal("blank runtime_gate annotation must not report a gate")
	}
}
