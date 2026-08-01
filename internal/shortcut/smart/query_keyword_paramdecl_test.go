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

package smart

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func TestCrossPlatformCoverageQueryToKeywordParamDecls(t *testing.T) {
	// Execute paths that feed --query into MCP "keyword" must declare the
	// mapping; flag_name_inference alone would publish property=query.
	for _, tc := range []struct {
		name string
		sc   shortcut.Shortcut
	}{
		{"+find-doc", FindDoc},
		{"+find-file", FindFile},
		{"+find-mail-user", FindMailUser},
		{"+minutes-search", MinutesSearch},
		{"+search-msg", SearchMsg},
		{"+find-record", FindRecord},
	} {
		var found bool
		for _, p := range tc.sc.Schema.Parameters {
			if p.Name == "query" && p.Property == "keyword" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s missing ParamDecl query→keyword; got %#v",
				tc.name, tc.sc.Schema.Parameters)
		}
	}
}
