// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package helpers

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestCrossPlatformCoverageDingPersonalRemindTypeMatchesMCPEnum(t *testing.T) {
	for input, want := range map[string]string{"app": "APP", "sms": "SMS", "call": "PHONE", " APP ": "APP"} {
		got, err := dingPersonalRemindType(input)
		if err != nil || got != want {
			t.Errorf("dingPersonalRemindType(%q)=(%q,%v), want %q", input, got, err, want)
		}
	}
	if got, err := dingPersonalRemindType("push"); err == nil || got != "" {
		t.Fatalf("unsupported remind type=(%q,%v)", got, err)
	}
}

func TestCrossPlatformCoverageResolveDingRobotCodeFailsClosed(t *testing.T) {
	t.Setenv("DINGTALK_DING_ROBOT_CODE", "")

	robotCode, err := resolveDingRobotCode("  ")
	if err == nil || robotCode != "" {
		t.Fatalf("resolveDingRobotCode() = (%q, %v), want typed failure", robotCode, err)
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) {
		t.Fatalf("error = %T, want *errors.Error", err)
	}
	if typed.Reason != "robot_credentials_missing" || typed.FailureStage != "preflight" {
		t.Fatalf("reason/stage = %q/%q", typed.Reason, typed.FailureStage)
	}
	if !typed.RetryableSet || typed.Retryable {
		t.Fatalf("retryability = (%v, %v), want explicit false", typed.RetryableSet, typed.Retryable)
	}
	if typed.ExecutionStarted == nil || *typed.ExecutionStarted {
		t.Fatalf("execution_started = %v, want false", typed.ExecutionStarted)
	}
	if len(typed.Actions) != 1 ||
		!strings.Contains(typed.Actions[0], "不要搜索 dev/devapp") ||
		!strings.Contains(typed.Actions[0], "禁止尝试或替换为其他机器人") {
		t.Fatalf("actions = %#v", typed.Actions)
	}
}

func TestCrossPlatformCoverageResolveDingRobotCodeUsesExplicitOrConfiguredValue(t *testing.T) {
	t.Setenv("DINGTALK_DING_ROBOT_CODE", " configured-robot ")

	if got, err := resolveDingRobotCode(" explicit-robot "); err != nil || got != "explicit-robot" {
		t.Fatalf("explicit robot = (%q, %v)", got, err)
	}
	if got, err := resolveDingRobotCode(""); err != nil || got != "configured-robot" {
		t.Fatalf("configured robot = (%q, %v)", got, err)
	}
}

func TestCrossPlatformCoverageDingRecallTargetRejectsChatResourceIDs(t *testing.T) {
	for _, id := range []string{"msgm9nQxDfZgPcvmF52jSUwAg==", "cidQDXPJySpuMWRqAH5LEc3Ig=="} {
		err := validateDingRecallTarget(id)
		if err == nil {
			t.Fatalf("validateDingRecallTarget(%q) unexpectedly succeeded", id)
		}
		var typed *apperrors.Error
		if !errors.As(err, &typed) || typed.Reason != "resource_type_mismatch" {
			t.Fatalf("validateDingRecallTarget(%q) = %#v, want resource_type_mismatch", id, err)
		}
		if typed.ExecutionStarted == nil || *typed.ExecutionStarted {
			t.Fatalf("validateDingRecallTarget(%q) execution_started=%v", id, typed.ExecutionStarted)
		}
	}
	for _, id := range []string{"78E3B7A70B89407BF371EF7DE295CFAB", "ding-fixture"} {
		if err := validateDingRecallTarget(id); err != nil {
			t.Fatalf("validateDingRecallTarget(%q) = %v", id, err)
		}
	}
}

func TestCrossPlatformCoverageDingConversionReceiptPreservesBothResourceIDs(t *testing.T) {
	got, err := enrichDingConversionReceipt(
		`{"success":true,"result":{"openDingId":" 78E3B7A70B89407BF371EF7DE295CFAB "}}`,
		" cidQDXPJySpuMWRqAH5LEc3Ig== ",
		" msgm9nQxDfZgPcvmF52jSUwAg== ",
	)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(got), &envelope); err != nil {
		t.Fatal(err)
	}
	result, _ := envelope["result"].(map[string]any)
	if result["openDingId"] != "78E3B7A70B89407BF371EF7DE295CFAB" ||
		result["sourceMessageId"] != "msgm9nQxDfZgPcvmF52jSUwAg==" ||
		result["sourceConversationId"] != "cidQDXPJySpuMWRqAH5LEc3Ig==" ||
		result["resourceType"] != "ding" {
		t.Fatalf("conversion result = %#v", result)
	}
	recall, _ := result["recallTarget"].(map[string]any)
	if recall["resourceType"] != "ding" || recall["openDingId"] != result["openDingId"] {
		t.Fatalf("recall target = %#v", recall)
	}

	if _, err := enrichDingConversionReceipt(`{"success":true,"result":{}}`, "cid", "msg"); err == nil {
		t.Fatal("missing openDingId unexpectedly produced a conversion receipt")
	}
}

func TestCrossPlatformCoverageDingProductDeclRoutesAllSupportedIdentities(t *testing.T) {
	_ = newDingCommand()
	decl, ok := contract.LookupProductDecl("ding")
	if !ok {
		t.Fatal("ding ProductDecl is not registered")
	}
	for _, required := range []string{
		"需要查询 DING 历史或接收状态",
		"需要以当前用户身份发送、消息转 DING 或撤回个人 DING",
		"明确指定企业机器人发送或撤回 DING",
	} {
		if !containsExact(decl.Selection.UseWhen, required) {
			t.Errorf("ding use_when = %#v, missing %q", decl.Selection.UseWhen, required)
		}
	}
	if len(decl.Selection.AvoidWhen) != 1 || !strings.Contains(decl.Selection.AvoidWhen[0], "普通聊天消息") {
		t.Fatalf("ding avoid_when = %#v, want explicit Chat ownership", decl.Selection.AvoidWhen)
	}
}

func containsExact(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
