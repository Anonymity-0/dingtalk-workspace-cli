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

package helpers

import (
	"bytes"
	"strings"
	"testing"
)

func TestSheetVersionRevertHelpDocumentsConfirmedRevisionTargets(t *testing.T) {
	command := newSheetVersionCmd()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"revert", "--help"})

	if err := command.Execute(); err != nil {
		t.Fatalf("sheet version revert --help returned error: %v", err)
	}
	help := output.String()
	for _, expected := range []string{
		"通常应从 version list 选择已保存的历史版本",
		"已从同一工作簿真实查询结果确认的 revision",
		"禁止猜测 revision",
		"目标历史版本或已确认 revision",
	} {
		if !strings.Contains(help, expected) {
			t.Fatalf("sheet version revert help missing %q:\n%s", expected, help)
		}
	}
}
