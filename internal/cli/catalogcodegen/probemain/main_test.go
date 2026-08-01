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

package main

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/catalogcodegen"
)

func TestCrossPlatformCoverageProbeMainPrintsToolTotals(t *testing.T) {
	if len(catalogcodegen.Tools) == 0 {
		t.Fatal("generated catalog tools must be non-empty")
	}
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	main()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	fields := strings.Fields(strings.TrimSpace(buf.String()))
	if len(fields) != 2 {
		t.Fatalf("main output = %q, want \"<tools> <total>\"", buf.String())
	}
	tools, err := strconv.Atoi(fields[0])
	if err != nil || tools != len(catalogcodegen.Tools) {
		t.Fatalf("tool count = %q, want %d", fields[0], len(catalogcodegen.Tools))
	}
}
