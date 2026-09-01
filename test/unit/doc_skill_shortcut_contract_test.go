package unit_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDocSkillPinsRequiredShortcutArguments(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	path := filepath.Join(root, "skills", "multi", "dingtalk-doc", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(content)

	for _, required := range []string{
		"dws doc +list --folder <ID或URL> --page-all",
		"返回真实 `name/nodeType/nodeId/url`",
		"`+fetch` 若报目标为目录",
		"禁止切 `drive +list`",
		"dws doc +create --name <标题> --content <文本\\|-\\|@文件> [--folder <ID>\\|--workspace <ID>]",
		"dws doc +import --file <相对路径> [--folder <ID>\\|--workspace <ID>]",
		"指定位置复用真实 ID，二者互斥",
		"未指定才由 Runtime 取默认根",
		"dws doc +comment-create --node <ID或URL> --content <文字> [--selection <原文>]",
		"+review --node <ID或URL>",
		"node/content 必填",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("%s missing required shortcut contract %q", path, required)
		}
	}

	referencePath := filepath.Join(root, "skills", "multi", "dingtalk-doc", "references", "doc.md")
	reference, err := os.ReadFile(referencePath)
	if err != nil {
		t.Fatalf("read %s: %v", referencePath, err)
	}
	if !strings.Contains(string(reference), "dws doc +list --folder <ID或URL> --page-all") {
		t.Errorf("%s does not preserve the alidocs document-directory route", referencePath)
	}
}
