// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProtectedReferences(t *testing.T) {
	root := t.TempDir()
	relative := "skills/multi/dingtalk-doc/references/doc/style/example.md"
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("stable\n")
	if err := os.WriteFile(absolute, content, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := routeManifest{
		ProtectedReferenceRoots: []string{"skills/multi/dingtalk-doc/references/doc/style"},
		ProtectedReferenceSHA256: map[string]string{
			relative: fmt.Sprintf("%x", sha256.Sum256(content)),
		},
	}
	if failures := validateProtectedReferences(root, manifest); len(failures) != 0 {
		t.Fatalf("valid protected references failed: %v", failures)
	}

	if err := os.WriteFile(absolute, []byte("drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	failures := validateProtectedReferences(root, manifest)
	if !containsFailure(failures, "sha256") {
		t.Fatalf("hash drift failures = %v", failures)
	}

	extra := filepath.Join(filepath.Dir(absolute), "unreviewed.md")
	if err := os.WriteFile(extra, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	failures = validateProtectedReferences(root, manifest)
	if !containsFailure(failures, "unreviewed file") {
		t.Fatalf("unreviewed file failures = %v", failures)
	}
}

func containsFailure(failures []string, needle string) bool {
	for _, failure := range failures {
		if strings.Contains(failure, needle) {
			return true
		}
	}
	return false
}
