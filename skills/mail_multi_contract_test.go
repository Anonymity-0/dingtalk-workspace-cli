// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package skills

import (
	"strings"
	"testing"
)

func TestMailMultiDestructiveAndAttachmentContracts(t *testing.T) {
	data, err := FS.ReadFile("multi/dingtalk-mail/references/09-mail.md")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	confirmation := strings.Index(content, "确认前禁止调用删除命令")
	deleteCall := strings.Index(content, "mail contact batch-delete --email <邮箱>")
	if confirmation < 0 || deleteCall < 0 || confirmation > deleteCall {
		t.Fatal("mail contact deletion must stop for confirmation before showing the executable delete call")
	}
	for _, required := range []string{
		"仅用已展示且获确认的 ID",
		"沿 `nextCursor` 遍历全部匹配页",
		"不能断言不存在",
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("mail multi contract missing %q", required)
		}
	}
}
