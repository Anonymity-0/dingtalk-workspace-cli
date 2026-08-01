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
	coreann "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/runtimeannotate"
)

const (
	AnnotationProduct      = coreann.AnnotationProduct
	AnnotationTool         = coreann.AnnotationTool
	AnnotationSource       = coreann.AnnotationSource
	AnnotationTitle        = coreann.AnnotationTitle
	AnnotationDescription  = coreann.AnnotationDescription
	AnnotationMetaSource   = coreann.AnnotationMetaSource
	AnnotationExclude      = coreann.AnnotationExclude
	AnnotationConstraints  = coreann.AnnotationConstraints
	AnnotationPositionals  = coreann.AnnotationPositionals
	AnnotationContract     = coreann.AnnotationContract
	AnnotationRisk         = coreann.AnnotationRisk
	AnnotationRuntimeGate  = coreann.AnnotationRuntimeGate
	AnnotationFlagProperty = coreann.AnnotationFlagProperty
	AnnotationFlagType     = coreann.AnnotationFlagType
	AnnotationFlagRequired = coreann.AnnotationFlagRequired
	AnnotationFlagReqWhen  = coreann.AnnotationFlagReqWhen
	AnnotationFlagExample  = coreann.AnnotationFlagExample
	AnnotationFlagFormat   = coreann.AnnotationFlagFormat
	AnnotationFlagEnum     = coreann.AnnotationFlagEnum
)

type RuntimeSchemaConstraints = coreann.RuntimeSchemaConstraints

var (
	AttachRuntimeSchema              = coreann.AttachRuntimeSchema
	AnnotateRuntimeToolMetadata      = coreann.AnnotateRuntimeToolMetadata
	AnnotateRuntimeFlag              = coreann.AnnotateRuntimeFlag
	AnnotateRuntimeFlagProperty      = coreann.AnnotateRuntimeFlagProperty
	AnnotateRuntimeRequiredFlags     = coreann.AnnotateRuntimeRequiredFlags
	AnnotateRuntimeFlagRequiredValue = coreann.AnnotateRuntimeFlagRequiredValue
	AnnotateRuntimeFlagDescription   = coreann.AnnotateRuntimeFlagDescription
	AnnotateRuntimeFlagRequiredWhen  = coreann.AnnotateRuntimeFlagRequiredWhen
	AnnotateRuntimeFlagFormat        = coreann.AnnotateRuntimeFlagFormat
	AnnotateRuntimeFlagInterfaceType = coreann.AnnotateRuntimeFlagInterfaceType
	AnnotateRuntimeFlagEnum          = coreann.AnnotateRuntimeFlagEnum
	AnnotateRuntimeFlagExample       = coreann.AnnotateRuntimeFlagExample
	AnnotateRuntimeContract          = coreann.AnnotateRuntimeContract
	AnnotateRuntimeRisk              = coreann.AnnotateRuntimeRisk
	AnnotateRuntimeGate              = coreann.AnnotateRuntimeGate
	RuntimeContractRisk              = coreann.RuntimeContractRisk
	RuntimeContractGate              = coreann.RuntimeContractGate
	AnnotateRuntimeConstraints       = coreann.AnnotateRuntimeConstraints
	CommandConstraints               = coreann.CommandConstraints
	NormalizeConstraints             = coreann.NormalizeConstraints
	ConstraintsEmpty                 = coreann.ConstraintsEmpty
	AnnotateRuntimePositionals       = coreann.AnnotateRuntimePositionals
	CommandPositionals               = coreann.CommandPositionals
	ExcludeFromRuntimeSchema         = coreann.ExcludeFromRuntimeSchema
	CommandFlag                      = coreann.CommandFlag
	SetCommandAnnotation             = coreann.SetCommandAnnotation
	SetFlagAnnotation                = coreann.SetFlagAnnotation
	SetFlagAnnotationValues          = coreann.SetFlagAnnotationValues
)
