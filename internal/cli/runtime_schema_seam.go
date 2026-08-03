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
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/runtimeannotate"
)

// The declaration model and its runtime stores live under internal/corecmd/*
// and every package imports them directly from there. This file only keeps
// the cli.* convenience aliases so the delivery root's own code reads flat;
// no separate cli shim packages exist.

type RuntimeSchemaConstraints = runtimeannotate.RuntimeSchemaConstraints

var (
	AttachRuntimeSchema                 = runtimeannotate.AttachRuntimeSchema
	AnnotateRuntimeToolMetadata         = runtimeannotate.AnnotateRuntimeToolMetadata
	AnnotateRuntimeFlag                 = runtimeannotate.AnnotateRuntimeFlag
	AnnotateRuntimeFlagProperty         = runtimeannotate.AnnotateRuntimeFlagProperty
	AnnotateRuntimeRequiredFlags        = runtimeannotate.AnnotateRuntimeRequiredFlags
	AnnotateRuntimeFlagRequiredValue    = runtimeannotate.AnnotateRuntimeFlagRequiredValue
	AnnotateRuntimeFlagDescription      = runtimeannotate.AnnotateRuntimeFlagDescription
	AnnotateRuntimeFlagRequiredWhen     = runtimeannotate.AnnotateRuntimeFlagRequiredWhen
	AnnotateRuntimeFlagFormat           = runtimeannotate.AnnotateRuntimeFlagFormat
	AnnotateRuntimeFlagInterfaceType    = runtimeannotate.AnnotateRuntimeFlagInterfaceType
	AnnotateRuntimeFlagEnum             = runtimeannotate.AnnotateRuntimeFlagEnum
	AnnotateRuntimeFlagExample          = runtimeannotate.AnnotateRuntimeFlagExample
	AnnotateRuntimeContract             = runtimeannotate.AnnotateRuntimeContract
	AnnotateRuntimeRisk                 = runtimeannotate.AnnotateRuntimeRisk
	AnnotateRuntimeGate                 = runtimeannotate.AnnotateRuntimeGate
	RuntimeContractRisk                 = runtimeannotate.RuntimeContractRisk
	RuntimeContractGate                 = runtimeannotate.RuntimeContractGate
	AnnotateRuntimeConstraints          = runtimeannotate.AnnotateRuntimeConstraints
	AnnotateRuntimePositionals          = runtimeannotate.AnnotateRuntimePositionals
	ExcludeFromRuntimeSchema            = runtimeannotate.ExcludeFromRuntimeSchema
	HasDeclaredOrAnnotatedConfirmation  = contractfinal.HasDeclaredOrAnnotatedConfirmation
	RuntimeContractFinal                = contractfinal.RuntimeContractFinal
	HasRuntimeContractFinal             = contractfinal.HasRuntimeContractFinal
	ClearRuntimeContractFinalForTest    = contractfinal.ClearRuntimeContractFinalForTest
	StoreRuntimeContractFinalRawForTest = contractfinal.StoreRuntimeContractFinalRawForTest
	ApplyParamDecls                     = contractfinal.ApplyParamDecls
)

func resolvedFieldProvenance(value any, source, sourceRef, precedence, resolution, reviewReason string) contract.FieldProvenance {
	return contract.ResolvedFieldProvenance(value, source, sourceRef, precedence, resolution, reviewReason)
}

func setRuntimeCommandAnnotation(cmd *cobra.Command, key, value string) {
	runtimeannotate.SetCommandAnnotation(cmd, key, value)
}

func setFlagAnnotation(flag *pflag.Flag, key, value string) {
	runtimeannotate.SetFlagAnnotation(flag, key, value)
}

func setFlagAnnotationValues(flag *pflag.Flag, key string, values ...string) {
	runtimeannotate.SetFlagAnnotationValues(flag, key, values...)
}
