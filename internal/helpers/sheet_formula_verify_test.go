package helpers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func executeFormulaVerify(t *testing.T, caller *scriptedToolCaller, stdin *strings.Reader, args ...string) error {
	t.Helper()
	installScriptedCaller(t, caller)
	cmd := newSheetFormulaVerifyCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if stdin != nil {
		cmd.SetIn(stdin)
	}
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestCrossPlatformCoverageSheetFormulaVerifyRejectsNonPositiveLimits(t *testing.T) {
	if err := executeFormulaVerify(t, &scriptedToolCaller{}, nil,
		"--node", "n1", "--max-locations-per-error", "0"); err == nil {
		t.Fatal("max-locations-per-error 0 returned nil")
	}
	if err := executeFormulaVerify(t, &scriptedToolCaller{}, nil,
		"--node", "n1", "--max-cells", "-1"); err == nil {
		t.Fatal("max-cells -1 returned nil")
	}
}

func TestCrossPlatformCoverageSheetFormulaVerifyLimitsAndInlineTargets(t *testing.T) {
	caller := &scriptedToolCaller{}
	err := executeFormulaVerify(t, caller, nil,
		"--node", "n1", "--max-locations-per-error", "3", "--max-cells", "100",
		"--targets", `[{"sheetId":"Sheet1","range":"A1:D10"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if caller.calls != 1 {
		t.Fatalf("calls = %d, want 1", caller.calls)
	}
}

func TestCrossPlatformCoverageSheetFormulaVerifyTargetsFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "targets.json")
	if err := os.WriteFile(path, []byte(`[{"sheetId":"Sheet1"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	caller := &scriptedToolCaller{}
	if err := executeFormulaVerify(t, caller, nil, "--node", "n1", "--targets", "@"+path); err != nil {
		t.Fatal(err)
	}
	if caller.calls != 1 {
		t.Fatalf("calls = %d, want 1", caller.calls)
	}
}

func TestCrossPlatformCoverageSheetFormulaVerifyTargetsFileMissing(t *testing.T) {
	err := executeFormulaVerify(t, &scriptedToolCaller{}, nil,
		"--node", "n1", "--targets", "@/nonexistent/targets.json")
	if err == nil || !strings.Contains(err.Error(), "读取 --targets 文件失败") {
		t.Fatalf("err = %v, want file read failure", err)
	}
}

func TestCrossPlatformCoverageSheetFormulaVerifyTargetsFromStdin(t *testing.T) {
	caller := &scriptedToolCaller{}
	if err := executeFormulaVerify(t, caller, strings.NewReader(`[{"sheetId":"S1"}]`),
		"--node", "n1", "--targets", "-"); err != nil {
		t.Fatal(err)
	}
	if caller.calls != 1 {
		t.Fatalf("calls = %d, want 1", caller.calls)
	}
}

func TestCrossPlatformCoverageSheetFormulaVerifyTargetsStdinFailure(t *testing.T) {
	installScriptedCaller(t, &scriptedToolCaller{})
	cmd := newSheetFormulaVerifyCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetIn(failingReader{})
	cmd.SetArgs([]string{"--node", "n1", "--targets", "-"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "读取 stdin 失败") {
		t.Fatalf("err = %v, want stdin failure", err)
	}
}
