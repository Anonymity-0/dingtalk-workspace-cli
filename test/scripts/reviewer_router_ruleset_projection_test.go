// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const strictUpdateRuleHelper = `function isStrictUpdateRule(rule) {
  if (rule?.type !== 'update') {
    return false;
  }
  const updateAllowsFetchAndMerge =
    rule.parameters?.update_allows_fetch_and_merge;
  return (
    updateAllowsFetchAndMerge === undefined ||
    updateAllowsFetchAndMerge === false
  );
}`

func TestReviewerRouterAcceptsGitHubStrictUpdateReadProjection(t *testing.T) {
	t.Parallel()

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required to verify the ruleset projection helper")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	var scripts []string
	for _, relativePath := range []string{
		filepath.Join(".github", "workflows", "ci.yml"),
		filepath.Join(".github", "workflows", "reviewer-router.yml"),
	} {
		path := filepath.Join(root, relativePath)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		var workflow any
		if unmarshalErr := yaml.Unmarshal(data, &workflow); unmarshalErr != nil {
			t.Fatalf("parse %s: %v", path, unmarshalErr)
		}
		collectGitHubScripts(workflow, &scripts)
	}

	checked := 0
	for _, script := range scripts {
		if !strings.Contains(script, "const writerRuleset = writerRulesets[0];") {
			continue
		}
		checked++
		if got := strings.Count(script, strictUpdateRuleHelper); got != 1 {
			t.Errorf("writer-ruleset script contains strict update helper %d times, want 1", got)
		}
		if strings.Contains(
			script,
			"parameters?.update_allows_fetch_and_merge !== false",
		) {
			t.Error("writer-ruleset script rejects GitHub's omitted strict-false read projection")
		}
		if !strings.Contains(script, "!isStrictUpdateRule(writerRuleset.rules[0])") {
			t.Error("writer-ruleset script does not enforce the strict update helper")
		}
	}
	if checked != 3 {
		t.Fatalf("writer-ruleset scripts checked = %d, want App enable, reconcile, and CI self-check", checked)
	}

	verification := strictUpdateRuleHelper + `
const cases = [
  ['omitted parameters', {type: 'update'}, true],
  ['null parameters', {type: 'update', parameters: null}, true],
  ['empty parameters', {type: 'update', parameters: {}}, true],
  ['explicit false', {type: 'update', parameters: {update_allows_fetch_and_merge: false}}, true],
  ['explicit true', {type: 'update', parameters: {update_allows_fetch_and_merge: true}}, false],
  ['null field', {type: 'update', parameters: {update_allows_fetch_and_merge: null}}, false],
  ['string false', {type: 'update', parameters: {update_allows_fetch_and_merge: 'false'}}, false],
  ['numeric false', {type: 'update', parameters: {update_allows_fetch_and_merge: 0}}, false],
  ['wrong rule type', {type: 'creation'}, false],
  ['null rule', null, false],
];
for (const [name, rule, want] of cases) {
  const got = isStrictUpdateRule(rule);
  if (got !== want) {
    throw new Error(name + ': got ' + got + ', want ' + want);
  }
}
`
	command := exec.Command(node, "-e", verification)
	if output, runErr := command.CombinedOutput(); runErr != nil {
		t.Fatalf("strict update projection verification failed: %v\n%s", runErr, output)
	}
}
