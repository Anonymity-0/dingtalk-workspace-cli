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

package contact

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func TestCrossPlatformCoverageQueryToKeywordParamDecls(t *testing.T) {
	assertQueryKeywordParamDecl(t, "+search-user", SearchUser)
	if len(shortcut.FromShortcut(SearchUser).Schema.Parameters) == 0 {
		t.Fatal("+search-user Schema.Parameters empty after FromShortcut")
	}
}

func assertQueryKeywordParamDecl(t *testing.T, name string, sc shortcut.Shortcut) {
	t.Helper()
	for _, p := range sc.Schema.Parameters {
		if p.Name == "query" && p.Property == "keyword" {
			return
		}
	}
	t.Fatalf("%s missing ParamDecl query→keyword; got %#v", name, sc.Schema.Parameters)
}
