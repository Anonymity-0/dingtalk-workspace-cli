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

package cmdcore

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestCrossPlatformCoverageNewCommandNilDispatch exercises the generated RunE
// path when a CommandSpec declares neither RunE nor Dispatch: after the shared
// validation/confirm pipeline it must no-op (return nil). FromLeafSpec always
// supplies a Dispatch or RunE, so this branch is only reachable by a bare
// CommandSpec built directly on the unified base.
func TestCrossPlatformCoverageNewCommandNilDispatch(t *testing.T) {
	cmd := NewCommand(CommandSpec{
		Use:   "bare",
		Short: "no dispatch",
		Flags: []FlagSpec{{Name: "x", Usage: "X"}},
		// Risk defaults to read → ConfirmRisk passes without prompting.
	})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--x", "v"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("nil-dispatch command should no-op, got %v", err)
	}
}

// TestCrossPlatformCoverageNewCommandDispatch confirms the assembled toolArgs
// reach Dispatch and that Dispatch's result is returned.
func TestCrossPlatformCoverageNewCommandDispatch(t *testing.T) {
	var got map[string]any
	cmd := NewCommand(CommandSpec{
		Use:   "route",
		Flags: []FlagSpec{{Name: "name", Usage: "N", Bind: "userName"}},
		Dispatch: func(_ *cobra.Command, _ []string, toolArgs map[string]any) error {
			got = toolArgs
			return nil
		},
	})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--name", "alice"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got["userName"] != "alice" {
		t.Fatalf("dispatch toolArgs = %#v", got)
	}
}
