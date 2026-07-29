// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handlers

import (
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/pipeline"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/cmdutil"
)

// BoolValueHandler makes an explicit, whitespace-separated boolean value
// unambiguous before pflag parsing. pflag interprets `--yes false` as a bare
// `--yes` (true) plus a positional `false`; rewriting the pair to
// `--yes=false` preserves the value the caller actually supplied.
//
// This handler runs after all flag-name handlers so camelCase, semantic
// aliases, and conservative fuzzy corrections have already reached their
// canonical names. It consumes only exact boolean literals and stops at `--`.
type BoolValueHandler struct{}

func (BoolValueHandler) Name() string          { return "boolvalue" }
func (BoolValueHandler) Phase() pipeline.Phase { return pipeline.PreParse }

func (BoolValueHandler) Handle(ctx *pipeline.Context) error {
	if len(ctx.Args) < 2 || len(ctx.FlagSpecs) == 0 {
		return nil
	}

	tokens := make(map[string]pipeline.FlagInfo)
	for _, spec := range ctx.FlagSpecs {
		if spec.Name == "" || (spec.Type != "bool" && spec.Type != "boolean") {
			continue
		}
		tokens["--"+spec.Name] = spec
		if spec.Shorthand != "" {
			tokens["-"+spec.Shorthand] = spec
		}
	}

	result := make([]string, 0, len(ctx.Args))
	for index := 0; index < len(ctx.Args); index++ {
		argument := ctx.Args[index]
		if argument == "--" {
			result = append(result, ctx.Args[index:]...)
			break
		}

		spec, matched := tokens[argument]
		if !matched || ctx.IsFlagProtected(spec.Name) || index+1 >= len(ctx.Args) {
			result = append(result, argument)
			continue
		}
		normalized, ok := cmdutil.NormalizeBoolLiteral(ctx.Args[index+1])
		if !ok {
			result = append(result, argument)
			continue
		}

		corrected := "--" + spec.Name + "=" + normalized
		original := strings.Join(ctx.Args[index:index+2], " ")
		ctx.AddCorrection("boolvalue", pipeline.PreParse, "--"+spec.Name, original, corrected, "explicit-bool")
		result = append(result, corrected)
		index++
	}

	ctx.Args = result
	return nil
}
