// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package localio

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func TestCrossPlatformCoverageReadTextInputUsesOpenedDescriptorAndBoundedReadE2E(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	t.Run("path replacement", func(t *testing.T) {
		path := filepath.Join(dir, "input.txt")
		replacement := filepath.Join(dir, "replacement.txt")
		if err := os.WriteFile(path, []byte("safe"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(replacement, []byte("replacement-exceeds-limit"), 0o600); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &openTextInputFile, func(candidate string) (*os.File, error) {
			file, openErr := os.Open(candidate)
			if openErr != nil {
				return nil, openErr
			}
			if renameErr := os.Rename(replacement, candidate); renameErr != nil {
				_ = file.Close()
				return nil, renameErr
			}
			return file, nil
		})
		got, err := ReadTextInput("@input.txt", nil, 10)
		if err != nil || got != "safe" {
			t.Fatalf("replacement read = %q, %v", got, err)
		}
	})

	t.Run("file grows after stat", func(t *testing.T) {
		path := filepath.Join(dir, "grow.txt")
		if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &readTextInputAll, func(reader io.Reader) ([]byte, error) {
			file, openErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if openErr != nil {
				return nil, openErr
			}
			if _, writeErr := file.WriteString("-too-large"); writeErr != nil {
				_ = file.Close()
				return nil, writeErr
			}
			if closeErr := file.Close(); closeErr != nil {
				return nil, closeErr
			}
			return io.ReadAll(reader)
		})
		if _, err := ReadTextInput("@grow.txt", nil, 4); err == nil {
			t.Fatal("file growth bypassed size limit")
		}
	})
}
