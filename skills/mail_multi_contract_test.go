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

	for _, required := range []string{
		"确认前禁止调用删除命令",
		"仅用已展示且获确认的 ID",
		"mail contact batch-delete --email <邮箱> --contact-ids <confirmed-id1,confirmed-id2,...> --yes",
		"沿 `nextCursor` 遍历全部匹配页",
		"不能断言不存在",
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("mail multi contract missing %q", required)
		}
	}
}
