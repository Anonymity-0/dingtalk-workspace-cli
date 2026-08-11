// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvalDispatchWorkflowUsesRepositoryPermissionAndReviewedSHA(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "eval-dispatch.yml"))
	if err != nil {
		t.Fatalf("read eval-dispatch workflow: %v", err)
	}
	workflow := string(data)

	for _, want := range []string{
		"/collaborators/${COMMENTER}/permission",
		"eval_dispatch_guard.py permission",
		"REVIEWED_SHA: ${{ steps.parse.outputs.reviewed_sha }}",
		"eval_dispatch_guard.py head",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("eval-dispatch workflow missing security contract %q", want)
		}
	}
	if strings.Contains(workflow, "author_association") {
		t.Error("eval-dispatch workflow must not authorize from author_association")
	}
}

func TestEvalDispatchRejectsLowRepositoryPermissions(t *testing.T) {
	t.Parallel()

	for _, permission := range []string{"read", "triage", "none"} {
		permission := permission
		t.Run(permission, func(t *testing.T) {
			t.Parallel()
			output, err := runEvalDispatchGuard(t, "permission", `{"permission":"`+permission+`"}`, "COMMENTER=low-privilege-user")
			if err == nil {
				t.Fatalf("permission %q unexpectedly allowed; output=%s", permission, output)
			}
			if !strings.Contains(output, "does not have write, maintain, or admin permission") {
				t.Fatalf("permission %q rejection = %q, want trusted-permission error", permission, output)
			}
		})
	}
}

func TestEvalDispatchAllowsTrustedRepositoryPermissions(t *testing.T) {
	t.Parallel()

	for _, permission := range []string{"write", "maintain", "admin"} {
		permission := permission
		t.Run(permission, func(t *testing.T) {
			t.Parallel()
			output, err := runEvalDispatchGuard(t, "permission", `{"permission":"`+permission+`"}`, "COMMENTER=maintainer")
			if err != nil {
				t.Fatalf("permission %q rejected: %v\n%s", permission, err, output)
			}
		})
	}
}

func TestEvalDispatchRejectsChangedPRHead(t *testing.T) {
	t.Parallel()

	const reviewedSHA = "1111111111111111111111111111111111111111"
	const currentSHA = "2222222222222222222222222222222222222222"
	pr := `{"number":934,"state":"open","head":{"sha":"` + currentSHA + `"}}`
	output, err := runEvalDispatchGuard(
		t,
		"head",
		pr,
		"EXPECTED_PR_NUMBER=934",
		"REVIEWED_SHA="+reviewedSHA,
	)
	if err == nil {
		t.Fatalf("changed PR head unexpectedly allowed; output=%s", output)
	}
	if !strings.Contains(output, "PR head changed after review") {
		t.Fatalf("changed-head rejection = %q, want explicit stale-review error", output)
	}
}

func TestEvalDispatchAcceptsReviewedCurrentPRHead(t *testing.T) {
	t.Parallel()

	const reviewedSHA = "1111111111111111111111111111111111111111"
	pr := `{"number":934,"state":"open","head":{"sha":"` + reviewedSHA + `"}}`
	output, err := runEvalDispatchGuard(
		t,
		"head",
		pr,
		"EXPECTED_PR_NUMBER=934",
		"REVIEWED_SHA="+reviewedSHA,
	)
	if err != nil {
		t.Fatalf("reviewed current PR head rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "head_sha="+reviewedSHA) {
		t.Fatalf("head guard output = %q, want pinned head SHA", output)
	}
}

func TestEvalCommentRequiresExplicitReviewedSHA(t *testing.T) {
	t.Parallel()

	const reviewedSHA = "1111111111111111111111111111111111111111"
	output, err := runEvalCommentParser(t, "/eval drive sha="+reviewedSHA)
	if err != nil {
		t.Fatalf("explicit reviewed SHA rejected: %v\n%s", err, output)
	}
	for _, want := range []string{"products=drive", "reviewed_sha=" + reviewedSHA} {
		if !strings.Contains(output, want) {
			t.Errorf("parser output = %q, want %q", output, want)
		}
	}

	output, err = runEvalCommentParser(t, "/eval drive")
	if err == nil {
		t.Fatalf("command without reviewed SHA unexpectedly allowed; output=%s", output)
	}
	if !strings.Contains(output, "审核 SHA 显式必填") {
		t.Fatalf("missing-SHA rejection = %q, want explicit reviewed-SHA error", output)
	}
}

func runEvalDispatchGuard(t *testing.T, mode, input string, env ...string) (string, error) {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	cmd := exec.Command("python3", filepath.Join(root, "scripts", "ci", "eval_dispatch_guard.py"), mode)
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), env...)
	output, runErr := cmd.CombinedOutput()
	return string(output), runErr
}

func runEvalCommentParser(t *testing.T, comment string) (string, error) {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	outputPath := filepath.Join(t.TempDir(), "github-output")
	cmd := exec.Command("python3", filepath.Join(root, "scripts", "ci", "eval_comment_parse.py"))
	cmd.Env = append(
		os.Environ(),
		"COMMENT_BODY="+comment,
		"GITHUB_OUTPUT="+outputPath,
	)
	combined, runErr := cmd.CombinedOutput()
	fileOutput, readErr := os.ReadFile(outputPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatalf("read parser GitHub output: %v", readErr)
	}
	return string(combined) + string(fileOutput), runErr
}
