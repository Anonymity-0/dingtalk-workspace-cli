package helpers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempImage(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("fake-image-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCrossPlatformCoverageValidateCoverImageFile(t *testing.T) {
	if _, _, err := validateCoverImageFile(t.TempDir()); err == nil {
		t.Fatal("directory returned nil")
	}
	path := writeTempImage(t, "cover.png")
	mimeType, size, err := validateCoverImageFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/png" || size != int64(len("fake-image-bytes")) {
		t.Fatalf("mimeType=%q size=%d", mimeType, size)
	}
}

func TestCrossPlatformCoverageIsHexColorEdge(t *testing.T) {
	for _, valid := range []string{"#E8F2FE", "#e8f2fe", "#000000", "#aB1234"} {
		if !isHexColor(valid) {
			t.Fatalf("%s should be valid", valid)
		}
	}
	for _, invalid := range []string{"#GG0000", "E8F2FE", "#E8F2F", "#E8F2FEE", ""} {
		if isHexColor(invalid) {
			t.Fatalf("%s should be invalid", invalid)
		}
	}
}

func executeDocStyleCommand(t *testing.T, caller *scriptedToolCaller, args ...string) error {
	t.Helper()
	installScriptedCaller(t, caller)
	root := newDocStyleCommand()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetArgs(args)
	return root.Execute()
}

func TestCrossPlatformCoverageDocStyleCoverSetMutualExclusion(t *testing.T) {
	err := executeDocStyleCommand(t, &scriptedToolCaller{},
		"cover", "set", "--node", "n1", "--image", "https://img.test/a.png", "--file-path", "a.png")
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("err = %v, want mutual exclusion", err)
	}
}

func TestCrossPlatformCoverageDocStyleCoverSetPositionValidation(t *testing.T) {
	err := executeDocStyleCommand(t, &scriptedToolCaller{},
		"cover", "set", "--node", "n1", "--image", "https://img.test/a.png", "--position", "1.5")
	if err == nil || !strings.Contains(err.Error(), "--position must be within [0,1]") {
		t.Fatalf("err = %v, want position range error", err)
	}

	caller := &scriptedToolCaller{}
	if err := executeDocStyleCommand(t, caller,
		"cover", "set", "--node", "n1", "--image", "https://img.test/a.png", "--position", "0.5"); err != nil {
		t.Fatal(err)
	}
	if caller.calls != 1 {
		t.Fatalf("calls = %d, want 1", caller.calls)
	}
}

func TestCrossPlatformCoverageDocStyleCoverSetFileDryRun(t *testing.T) {
	path := writeTempImage(t, "cover.png")
	caller := &scriptedToolCaller{dry: true}
	if err := executeDocStyleCommand(t, caller, "cover", "set", "--node", "n1", "--file", path); err != nil {
		t.Fatal(err)
	}
}

func stubHTTPPutFile(t *testing.T, err error) {
	t.Helper()
	old := httpPutFile
	httpPutFile = func(context.Context, string, map[string]string, string, int64) error { return err }
	t.Cleanup(func() { httpPutFile = old })
}

func TestCrossPlatformCoverageDocStyleCoverSetFileUpload(t *testing.T) {
	path := writeTempImage(t, "cover.png")
	stubHTTPPutFile(t, nil)
	caller := &scriptedToolCaller{steps: []scriptedToolStep{
		{text: `{"uploadUrl":"https://oss.test/put","resourceId":"res-1"}`},
		{text: `{}`},
	}}
	if err := executeDocStyleCommand(t, caller, "cover", "set", "--node", "n1", "--file", path); err != nil {
		t.Fatal(err)
	}
	if caller.calls != 2 {
		t.Fatalf("calls = %d, want 2 (upload info + style set)", caller.calls)
	}
}

func TestCrossPlatformCoverageDocStyleCoverSetFileUploadFailure(t *testing.T) {
	path := writeTempImage(t, "cover.png")
	stubHTTPPutFile(t, errors.New("oss down"))
	caller := &scriptedToolCaller{steps: []scriptedToolStep{
		{text: `{"uploadUrl":"https://oss.test/put","resourceId":"res-1"}`},
	}}
	err := executeDocStyleCommand(t, caller, "cover", "set", "--node", "n1", "--file", path)
	if err == nil || !strings.Contains(err.Error(), "oss down") {
		t.Fatalf("err = %v, want oss failure", err)
	}
}

func TestCrossPlatformCoverageUploadDocStyleImageFailures(t *testing.T) {
	caller := &scriptedToolCaller{steps: []scriptedToolStep{{err: errors.New("cred failed")}}}
	installScriptedCaller(t, caller)
	if _, err := uploadDocStyleImage(context.Background(), "n1", "a.png", "a.png", "image/png", 10); err == nil {
		t.Fatal("cred failure returned nil")
	}

	caller = &scriptedToolCaller{steps: []scriptedToolStep{{text: `not json`}}}
	installScriptedCaller(t, caller)
	if _, err := uploadDocStyleImage(context.Background(), "n1", "a.png", "a.png", "image/png", 10); err == nil {
		t.Fatal("invalid upload info returned nil")
	}
}
