// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package smart

import (
	"strings"
	"time"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func defaultChatPageLimit(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func validChatTime(value string) bool {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

func localChatOptionError(reason, message string, flags ...string) error {
	action := "修正参数后重试，或查看当前命令帮助"
	if flagText := strings.Join(flags, "、"); flagText != "" {
		action = "检查 " + flagText + " 后重试，或查看当前命令帮助"
	}
	return apperrors.NewValidation(
		message,
		apperrors.WithReason(reason),
		apperrors.WithActions(action),
	)
}
