// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package smart

import (
	"strings"
	"testing"
)

func TestRemindShortcutDoesNotAdvertiseDueTimeAsReminder(t *testing.T) {
	if strings.Contains(Remind.Description, "提醒时间") || strings.Contains(Remind.Intent, "截止/提醒") {
		t.Fatalf("shortcut contract still conflates dueTime with a reminder: %#v", Remind)
	}
	for _, flag := range Remind.Flags {
		if flag.Name == "at" && !strings.Contains(flag.Desc, "不是提醒时间") {
			t.Fatalf("--at description = %q, want explicit dueTime boundary", flag.Desc)
		}
	}
}
