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
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestHasIntersection(t *testing.T) {
	tests := []struct {
		name string
		a, b []int64
		want bool
	}{
		{"both empty", nil, nil, false},
		{"a empty", nil, []int64{1}, false},
		{"b empty", []int64{1}, nil, false},
		{"no overlap", []int64{1, 2, 3}, []int64{4, 5, 6}, false},
		{"has overlap", []int64{1, 2, 3}, []int64{3, 4, 5}, true},
		{"single match", []int64{1}, []int64{1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasIntersection(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("hasIntersection(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsSystemBusy(t *testing.T) {
	if isSystemBusy(nil) {
		t.Error("isSystemBusy(nil) should be false")
	}
	if !isSystemBusy(errors.New("error code: SYSTEM_BUSY")) {
		t.Error("isSystemBusy should detect SYSTEM_BUSY in error message")
	}
	if isSystemBusy(errors.New("some other error")) {
		t.Error("isSystemBusy should return false for non-SYSTEM_BUSY errors")
	}
}

func TestParseExtension(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringArray("extension", nil, "")

	ext, err := parseExtension(cmd)
	if err != nil {
		t.Fatalf("empty extension should not error: %v", err)
	}
	if ext != nil {
		t.Fatalf("empty extension should return nil, got %v", ext)
	}

	_ = cmd.Flags().Set("extension", "key1=val1")
	_ = cmd.Flags().Set("extension", "key2=val2")
	ext, err = parseExtension(cmd)
	if err != nil {
		t.Fatalf("valid extension should not error: %v", err)
	}
	if ext["key1"] != "val1" || ext["key2"] != "val2" {
		t.Fatalf("unexpected extension map: %v", ext)
	}

	cmd2 := &cobra.Command{Use: "test2"}
	cmd2.Flags().StringArray("extension", nil, "")
	_ = cmd2.Flags().Set("extension", "badformat")
	_, err = parseExtension(cmd2)
	if err == nil {
		t.Fatal("invalid extension format should error")
	}
}

func TestToolbarConversationID(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("conversation-id", "", "")

	_, err := toolbarConversationID(cmd)
	if err == nil {
		t.Fatal("missing conversation-id should error")
	}

	_ = cmd.Flags().Set("conversation-id", "cid123")
	cid, err := toolbarConversationID(cmd)
	if err != nil {
		t.Fatalf("valid conversation-id should not error: %v", err)
	}
	if cid != "cid123" {
		t.Fatalf("expected cid123, got %s", cid)
	}
}
