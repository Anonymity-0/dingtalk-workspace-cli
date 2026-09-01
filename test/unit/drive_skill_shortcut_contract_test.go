package unit_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDriveSkillPinsDeleteNodeArgument(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	path := filepath.Join(root, "skills", "multi", "dingtalk-drive", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	required := "dws drive +delete --node <dentryUuid>"
	if !strings.Contains(string(content), required) {
		t.Fatalf("%s missing required shortcut contract %q", path, required)
	}
}

func TestDriveSkillRoutesAlidocsDocumentDirectoryToDocList(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	path := filepath.Join(root, "skills", "multi", "dingtalk-drive", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(content)
	for _, required := range []string{
		"浏览钉盘根目录或已知普通存储文件夹",
		"已知 alidocs 文档目录 URL 浏览子节点走 `doc +list`",
		"`doc +list --folder <URL>`",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("%s missing document-directory boundary %q", path, required)
		}
	}
}
