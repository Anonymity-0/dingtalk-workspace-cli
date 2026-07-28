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

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/agentproduct"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/configmeta"
)

func init() {
	configmeta.Register(configmeta.ConfigItem{
		Name:         agentproduct.EnvName,
		Category:     configmeta.CategoryExternal,
		Description:  "调用 DWS 的 Agent 产品标识；覆盖请求中的 claw-type",
		DefaultValue: "由当前发行版决定",
		Example:      "qwenwork",
	})
}

// parseAgentProduct converts the reusable package error into the CLI's stable
// structured validation error without exposing the untrusted raw value.
func parseAgentProduct(raw string) (string, error) {
	value, err := agentproduct.Parse(raw)
	if err != nil {
		return "", invalidAgentProductError()
	}
	return value, nil
}

func invalidAgentProductError() error {
	return apperrors.NewValidation(
		"DWS_AGENT_PRODUCT must match ^[A-Za-z0-9][A-Za-z0-9_-]*$",
		apperrors.WithReason("invalid_agent_product"),
	)
}

// applyAgentProductOverride applies a valid non-empty runtime override after
// edition hooks have supplied their existing default. Invalid values are
// ignored here to preserve the best-effort contract of library callers that
// bypass root command validation.
func applyAgentProductOverride(headers map[string]string) map[string]string {
	value, err := agentproduct.Parse(os.Getenv(agentproduct.EnvName))
	if err != nil || value == "" {
		return headers
	}
	if headers == nil {
		headers = make(map[string]string)
	}
	headers[agentproduct.HeaderName] = value
	return headers
}
