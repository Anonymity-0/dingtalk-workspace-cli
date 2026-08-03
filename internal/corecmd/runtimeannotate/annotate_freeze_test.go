// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package runtimeannotate_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Production must not grow new AnnotateRuntimeRisk / AnnotateRuntimeGate call
// sites. Prefer declared Contract SafetySpec. Existing definitions and the
// cli seam re-export remain; tests may still exercise the helpers.
func TestAnnotateRuntimeRiskAndGateHaveNoNewProductionCallSites(t *testing.T) {
	root := findModuleRoot(t)
	callPattern := regexp.MustCompile(`\b(?:runtimeannotate\.)?AnnotateRuntime(?:Risk|Gate)\s*\(`)
	allowed := map[string]bool{
		filepath.Join("internal", "corecmd", "runtimeannotate", "annotate.go"): true,
		filepath.Join("internal", "cli", "runtime_schema_seam.go"):             true,
	}
	var problems []string
	for _, dir := range []string{
		filepath.Join(root, "internal"),
		filepath.Join(root, "cmd"),
		filepath.Join(root, "pkg"),
	} {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				base := d.Name()
				if base == "testdata" || base == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if allowed[rel] {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, match := range callPattern.FindAllString(string(data), -1) {
				problems = append(problems, rel+": "+match)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	if len(problems) != 0 {
		t.Fatalf("new production AnnotateRuntimeRisk/Gate call sites are forbidden; migrate to Contract SafetySpec:\n - %s", strings.Join(problems, "\n - "))
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
