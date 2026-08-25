package scripts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDraftPRCIWorkflowContract(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	read := func(path string) string {
		t.Helper()
		data, readErr := os.ReadFile(filepath.Join(root, path))
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		return string(data)
	}

	admission := read(".github/workflows/ci.yml")
	for _, want := range []string{
		"ready_for_review",
		"converted_to_draft",
		"if: ${{ github.event_name == 'push' || github.event.pull_request.draft == false }}",
	} {
		if !strings.Contains(admission, want) {
			t.Errorf("formal CI workflow missing Draft boundary %q", want)
		}
	}
	jobsStart := strings.Index(admission, "\njobs:\n")
	if jobsStart < 0 {
		t.Fatal("formal CI workflow is missing jobs")
	}
	jobLines := strings.Split(admission[jobsStart+len("\njobs:\n"):], "\n")
	jobNames := make([]string, 0)
	jobStarts := make([]int, 0)
	for index, line := range jobLines {
		if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "    ") || !strings.HasSuffix(line, ":") {
			continue
		}
		jobNames = append(jobNames, strings.TrimSuffix(strings.TrimSpace(line), ":"))
		jobStarts = append(jobStarts, index)
	}
	if len(jobNames) == 0 || jobNames[0] != "lint" {
		t.Fatalf("formal CI first job = %v, want lint", jobNames)
	}
	for index := 1; index < len(jobNames); index++ {
		end := len(jobLines)
		if index+1 < len(jobStarts) {
			end = jobStarts[index+1]
		}
		body := strings.Join(jobLines[jobStarts[index]:end], "\n")
		if !strings.Contains(body, "\n    needs: lint\n") && !strings.Contains(body, "\n      - lint\n") {
			t.Errorf("formal CI job %q does not depend on the Draft-gated lint job", jobNames[index])
		}
	}

	aiBehavior := read(".github/workflows/ai-behavior-check.yml")
	for _, want := range []string{
		"ready_for_review",
		"converted_to_draft",
		"if: ${{ github.event_name == 'push' || github.event.pull_request.draft == false }}",
	} {
		if !strings.Contains(aiBehavior, want) {
			t.Errorf("AI Behavior workflow missing Draft boundary %q", want)
		}
	}

	draft := read(".github/workflows/draft-ci.yml")
	for _, want := range []string{
		"name: Draft CI",
		"types: [opened, synchronize, reopened, converted_to_draft, edited]",
		"format('noop-{0}', github.run_id)",
		"format('pr-{0}', github.event.pull_request.number)",
		"cancel-in-progress: true",
		"name: Draft Fast Gate",
		"github.event.pull_request.draft == true",
		"github.event.action != 'edited' || github.event.changes.base != null",
		`test "$(git rev-parse HEAD^1)" = "$PR_BASE_SHA"`,
		`test "$(git rev-parse HEAD^2)" = "$PR_HEAD_SHA"`,
		"run: make test-plan",
		"run: make format-check",
		"run: go vet ./...",
		"actionlint@v1.7.12",
		"run: make build",
		`git diff --check "$PR_BASE_SHA" HEAD`,
		`./scripts/policy/check-release-fragments.sh "$PR_BASE_SHA" HEAD`,
		"Draft Fast Gate is development feedback, not Code Admission.",
		"Mark this PR ready for review to run the nine required contexts.",
	} {
		if !strings.Contains(draft, want) {
			t.Errorf("Draft CI workflow missing contract %q", want)
		}
	}

	for _, forbidden := range []string{
		"pull_request_target:",
		"contents: write",
		"pull-requests: write",
		"checks: write",
		"go test -race",
		"make policy",
		"coverage-gate",
		"macos-latest",
		"windows-latest",
	} {
		if strings.Contains(draft, forbidden) {
			t.Errorf("Draft CI workflow contains admission-only behavior %q", forbidden)
		}
	}

	for _, context := range []string{
		"Lint",
		"Test",
		"Coverage",
		"Policy",
		"Edition",
		"Interface Integrity",
		"AI Behavior",
		"CLI Smoke",
		"Mock MCP",
	} {
		if strings.Contains(draft, "\n    name: "+context+"\n") {
			t.Errorf("Draft workflow must not emit formal context %q", context)
		}
	}
}
