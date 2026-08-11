// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package localio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrossPlatformCoverageReadTextInputLiteralStdinAndFileE2E(t *testing.T) {
	if got, err := ReadTextInput("literal", nil, 10); err != nil || got != "literal" {
		t.Fatalf("literal = %q, %v", got, err)
	}
	if got, err := ReadTextInput("-", strings.NewReader("stdin"), 10); err != nil || got != "stdin" {
		t.Fatalf("stdin = %q, %v", got, err)
	}
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.WriteFile(filepath.Join(dir, "input.json"), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadTextInput("@input.json", nil, 10); err != nil || got != "file" {
		t.Fatalf("file = %q, %v", got, err)
	}
}

func TestCrossPlatformCoverageReadTextInputRejectsEscapeAndOversizeE2E(t *testing.T) {
	for _, spec := range []string{"@../secret", "@/tmp/secret", "@"} {
		if _, err := ReadTextInput(spec, nil, 10); err == nil {
			t.Fatalf("unsafe input accepted: %q", spec)
		}
	}
	if _, err := ReadTextInput("-", strings.NewReader("too-large"), 2); err == nil {
		t.Fatal("oversize stdin accepted")
	}
}
