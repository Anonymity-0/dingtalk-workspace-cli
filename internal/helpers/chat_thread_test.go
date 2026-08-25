// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
	"github.com/spf13/pflag"
)

type chatThreadCall struct {
	product string
	tool    string
	args    map[string]any
}

type chatThreadCaller struct {
	calls     []chatThreadCall
	responses map[string]string
	errors    map[string]error
	dryRun    bool
}

func (c *chatThreadCaller) CallTool(_ context.Context, product, tool string, args map[string]any) (*edition.ToolResult, error) {
	c.calls = append(c.calls, chatThreadCall{product: product, tool: tool, args: args})
	if err := c.errors[product+"/"+tool]; err != nil {
		return nil, err
	}
	text := c.responses[product+"/"+tool]
	if text == "" {
		text = `{"success":true}`
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: text}}}, nil
}

func (*chatThreadCaller) Format() string { return "json" }
func (c *chatThreadCaller) DryRun() bool { return c.dryRun }
func (*chatThreadCaller) Fields() string { return "" }
func (*chatThreadCaller) JQ() string     { return "" }

func executeAtomicThreadCommand(t *testing.T, caller *chatThreadCaller, args ...string) error {
	t.Helper()
	testseam.Protect(t, &deps)
	InitDeps(caller)
	deps.Out.w = io.Discard
	deps.Out.errW = io.Discard
	root := newChatCommand()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	ctx, _ := output.WithResultStore(context.Background())
	executed, err := root.ExecuteContextC(ctx)
	if err != nil {
		return err
	}
	_, _, err = output.EmitStoredResult(executed)
	return err
}

func executeAtomicThreadDryRun(t *testing.T, caller *chatThreadCaller, args ...string) ([]byte, error) {
	t.Helper()
	testseam.Protect(t, &deps)
	InitDeps(caller)
	var stdout, stderr bytes.Buffer
	deps.Out.w = &stdout
	deps.Out.errW = &stderr
	root := newChatCommand()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	ctx, _ := output.WithResultStore(context.Background())
	executed, err := root.ExecuteContextC(ctx)
	if err != nil {
		return stdout.Bytes(), err
	}
	_, emitted, err := output.EmitStoredResult(executed)
	if err == nil && !emitted {
		err = errors.New("unified dry-run returned without a CommandResult")
	}
	return stdout.Bytes(), err
}

func TestCrossPlatformCoverageAtomicThreadDryRunStoresOneResult(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "fixture.txt")
	if err := os.WriteFile(filePath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "create", args: []string{"thread", "create-group", "--name", "话题圈", "--users", "user-1"}},
		{name: "send", args: []string{"thread", "send", "--conversation-id", "topic-1", "--text", "新话题"}},
		{name: "list", args: []string{"thread", "list", "--conversation-id", "topic-1"}},
		{name: "reply", args: []string{"thread", "reply", "--conversation-id", "thread-1", "--text", "回复"}},
		{name: "reply file", args: []string{"thread", "reply", "--conversation-id", "thread-1", "--msg-type", "file", "--file", filePath}},
		{name: "list-replies", args: []string{"thread", "list-replies", "--conversation-id", "topic-1", "--topic-id", "thread-1"}},
		{name: "forward", args: []string{"thread", "forward", "--src-msg-id", "message-1", "--src-conversation-id", "topic-1", "--src-thread-id", "thread-1", "--dest-conversation-id", "conversation-2"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := &chatThreadCaller{dryRun: true}
			stdout, err := executeAtomicThreadDryRun(t, caller, test.args...)
			if err != nil {
				t.Fatal(err)
			}
			var envelope map[string]any
			if err := json.Unmarshal(stdout, &envelope); err != nil {
				t.Fatalf("dry-run output is not one JSON result: %q: %v", stdout, err)
			}
			if envelope["dry_run"] != true || envelope["outcome"] != "success" {
				t.Fatalf("dry-run envelope = %#v", envelope)
			}
			if len(caller.calls) != 0 {
				t.Fatalf("dry-run calls = %#v", caller.calls)
			}
		})
	}
}

func TestCrossPlatformCoverageAtomicThreadCreatePropagatesBackendError(t *testing.T) {
	want := errors.New("create unavailable")
	caller := &chatThreadCaller{
		responses: map[string]string{
			"contact/get_current_user_profile": `{"result":{"userId":"owner-1"}}`,
		},
		errors: map[string]error{"im/create_group_conversation": want},
	}
	err := executeAtomicThreadCommand(t, caller,
		"thread", "create-group", "--name", "话题圈", "--users", "user-1")
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestCrossPlatformCoverageAtomicThreadListsPublishPaginationInMeta(t *testing.T) {
	for _, test := range []struct {
		name      string
		response  map[string]string
		args      []string
		wantItems float64
	}{
		{
			name: "topics",
			response: map[string]string{
				"chat/list_conversation_message_v2": `{"result":{"messages":[{"openMessageId":"root-1","openConvThreadId":"thread-1"}],"hasMore":true,"nextCursor":1787000000123}}`,
			},
			args:      []string{"thread", "list", "--conversation-id", "topic-1"},
			wantItems: 1,
		},
		{
			name: "topics filtered empty page",
			response: map[string]string{
				"chat/list_conversation_message_v2": `{"result":{"messages":[{"openMessageId":"ordinary-1"}],"hasMore":true,"nextCursor":1787000000123}}`,
			},
			args:      []string{"thread", "list", "--conversation-id", "topic-1"},
			wantItems: 0,
		},
		{
			name: "replies",
			response: map[string]string{
				"chat/list_topic_replies": `{"result":{"messages":[{"openMessageId":"reply-1"}],"hasMore":true,"nextCursor":1787000000123}}`,
			},
			args:      []string{"thread", "list-replies", "--conversation-id", "topic-1", "--topic-id", "thread-1"},
			wantItems: 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stdout, err := executeAtomicThreadDryRun(t, &chatThreadCaller{responses: test.response}, test.args...)
			if err != nil {
				t.Fatal(err)
			}
			var envelope map[string]any
			if err := json.Unmarshal(stdout, &envelope); err != nil {
				t.Fatal(err)
			}
			data, _ := envelope["data"].(map[string]any)
			for _, key := range []string{"hasMore", "nextCursor", "cursor", "nextPage", "complete"} {
				if _, exists := data[key]; exists {
					t.Fatalf("pagination field %q leaked into data: %#v", key, data)
				}
			}
			meta, _ := envelope["meta"].(map[string]any)
			pagination, _ := meta["pagination"].(map[string]any)
			gotItems, _ := pagination["items"].(float64)
			if pagination["endpoint_exhausted"] != false || pagination["next_token"] == "" || pagination["pages"] != float64(1) || gotItems != test.wantItems {
				t.Fatalf("pagination = %#v", pagination)
			}
		})
	}
}

func TestCrossPlatformCoverageAtomicThreadPaginationRejectsMissingCursor(t *testing.T) {
	caller := &chatThreadCaller{responses: map[string]string{
		"chat/list_conversation_message_v2": `{"result":{"messages":[{"openMessageId":"root-1","openConvThreadId":"thread-1"}],"hasMore":true}}`,
	}}
	err := executeAtomicThreadCommand(t, caller,
		"thread", "list", "--conversation-id", "topic-1")
	var typed *apperrors.Error
	if err == nil || !errors.As(err, &typed) || typed.Reason != "invalid_pagination" {
		t.Fatalf("error = %v", err)
	}
}

func TestCrossPlatformCoverageAtomicThreadRejectsNonJSONWithoutRawOutput(t *testing.T) {
	for _, test := range []struct {
		name      string
		responses map[string]string
		args      []string
	}{
		{
			name: "create",
			responses: map[string]string{
				"contact/get_current_user_profile": `{"result":{"userId":"owner-1"}}`,
				"im/create_group_conversation":     `<html>bad gateway</html>`,
			},
			args: []string{"thread", "create-group", "--name", "话题圈", "--users", "user-1"},
		},
		{
			name:      "list",
			responses: map[string]string{"chat/list_conversation_message_v2": `<html>bad gateway</html>`},
			args:      []string{"thread", "list", "--conversation-id", "topic-1"},
		},
		{
			name:      "list replies",
			responses: map[string]string{"chat/list_topic_replies": `<html>bad gateway</html>`},
			args:      []string{"thread", "list-replies", "--conversation-id", "topic-1", "--topic-id", "thread-1"},
		},
		{
			name:      "forward",
			responses: map[string]string{"im/forward_topic": `<html>bad gateway</html>`},
			args:      []string{"thread", "forward", "--src-msg-id", "message-1", "--src-conversation-id", "topic-1", "--src-thread-id", "thread-1", "--dest-conversation-id", "conversation-2"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stdout, err := executeAtomicThreadDryRun(t, &chatThreadCaller{responses: test.responses}, test.args...)
			var typed *apperrors.Error
			if err == nil || !errors.As(err, &typed) {
				t.Fatalf("error = %v, want structured response validation error", err)
			}
			if typed.FailureStage != "response_validation" || typed.Reason != "thread_response_invalid" {
				t.Fatalf("error = %#v", typed)
			}
			if len(stdout) != 0 {
				t.Fatalf("stdout = %q, want no raw response", stdout)
			}
		})
	}
}

func TestCrossPlatformCoverageChatThreadSurfaceAndLegacyCompatibility(t *testing.T) {
	root := newChatCommand()
	thread, remaining, err := root.Find([]string{"thread"})
	if err != nil || len(remaining) != 0 {
		t.Fatalf("find chat thread: command=%v remaining=%v error=%v", thread, remaining, err)
	}
	visible := map[string]bool{}
	for _, command := range thread.Commands() {
		if !command.Hidden {
			visible[command.Name()] = true
		}
	}
	want := map[string]bool{
		"create-group": true, "send": true, "list": true, "reply": true, "list-replies": true, "forward": true,
		"recall-message": true, "add-emoji": true, "remove-emoji": true,
		"list-emotion-replies": true, "add-text-emotion": true, "remove-text-emotion": true, "update-text-emotion": true,
	}
	if !reflect.DeepEqual(visible, want) {
		t.Fatalf("visible thread commands = %#v, want %#v", visible, want)
	}
	for _, path := range [][]string{{"message", "list-topic-replies"}, {"message", "forward-topic"}} {
		command, _, findErr := root.Find(path)
		if findErr != nil || !command.Hidden || !command.Runnable() {
			t.Fatalf("legacy path %v: command=%v hidden=%v runnable=%v error=%v", path, command, command != nil && command.Hidden, command != nil && command.Runnable(), findErr)
		}
	}
	create, _, err := root.Find([]string{"group", "create"})
	if err != nil || create.Flags().Lookup("thread") == nil || !create.Flags().Lookup("thread").Hidden {
		t.Fatalf("legacy --thread compatibility flag is not hidden: command=%v error=%v", create, err)
	}
	var help bytes.Buffer
	create.SetOut(&help)
	create.SetErr(&help)
	if err := create.Help(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(help.String(), "--thread") {
		t.Fatalf("chat group create help exposes legacy --thread:\n%s", help.String())
	}
	final, ok := contractfinal.RuntimeContractFinal(create)
	if !ok {
		t.Fatal("chat group create has no ContractFinal")
	}
	for _, parameter := range final.Parameters {
		if parameter.Name == "thread" {
			t.Fatalf("chat group create ContractFinal exposes legacy thread parameter: %#v", final.Parameters)
		}
	}
	threadCreate, _, err := root.Find([]string{"thread", "create-group"})
	if err != nil || !corecmd.InterfaceBoolConstParams(threadCreate)["convThreadEnabled"] {
		t.Fatalf("thread create const params = %#v, error=%v", corecmd.InterfaceBoolConstParams(threadCreate), err)
	}
	for _, paths := range []struct {
		legacy []string
		thread []string
	}{
		{legacy: []string{"message", "list"}, thread: []string{"thread", "list"}},
		{legacy: []string{"message", "list-topic-replies"}, thread: []string{"thread", "list-replies"}},
	} {
		legacy, _, legacyErr := root.Find(paths.legacy)
		threadCommand, _, threadErr := root.Find(paths.thread)
		if legacyErr != nil || threadErr != nil {
			t.Fatalf("find time-compatible commands: legacy=%v thread=%v", legacyErr, threadErr)
		}
		for _, name := range []string{"time", "direction"} {
			if legacy.Flags().Lookup(name).Usage != threadCommand.Flags().Lookup(name).Usage {
				t.Fatalf("%v --%s help = %q, want legacy %q", paths.thread, name, threadCommand.Flags().Lookup(name).Usage, legacy.Flags().Lookup(name).Usage)
			}
		}
	}
	legacySend, _, legacySendErr := root.Find([]string{"message", "send"})
	if legacySendErr != nil {
		t.Fatalf("find legacy message send: %v", legacySendErr)
	}
	for _, path := range [][]string{{"thread", "send"}, {"thread", "reply"}} {
		threadSend, _, threadSendErr := root.Find(path)
		if threadSendErr != nil {
			t.Fatalf("find %v: %v", path, threadSendErr)
		}
		for _, name := range []string{"content", "file", "at-all", "at-open-dingtalk-ids"} {
			if legacySend.Flags().Lookup(name).Usage != threadSend.Flags().Lookup(name).Usage {
				t.Fatalf("%v --%s help = %q, want legacy %q", path, name, threadSend.Flags().Lookup(name).Usage, legacySend.Flags().Lookup(name).Usage)
			}
		}
		for _, alias := range []string{"text", "body", "message", "markdown", "file-path", "uuid"} {
			flag := threadSend.Flags().Lookup(alias)
			if flag == nil || !flag.Hidden {
				t.Fatalf("%v --%s compatibility alias = %#v, want hidden", path, alias, flag)
			}
		}
	}
}

func TestCrossPlatformCoverageChatThreadKeepsPreSplitPrimaryParameters(t *testing.T) {
	root := newChatCommand()
	wantByLeaf := map[string][]string{
		"create-group":         {"name", "type", "users"},
		"send":                 {"ai-tag", "at-all", "at-open-dingtalk-ids", "content", "conversation-id", "file", "idempotency-key", "media-id", "msg-type", "title"},
		"list":                 {"conversation-id", "direction", "limit", "time"},
		"reply":                {"ai-tag", "at-all", "at-open-dingtalk-ids", "content", "conversation-id", "file", "idempotency-key", "media-id", "msg-type", "title"},
		"list-replies":         {"conversation-id", "direction", "limit", "time", "topic-id"},
		"forward":              {"dest-conversation-id", "src-conversation-id", "src-msg-id", "src-thread-id"},
		"recall-message":       {"conversation-id", "message-id"},
		"add-emoji":            {"conversation-id", "emoji", "message-id"},
		"remove-emoji":         {"conversation-id", "emoji", "message-id"},
		"list-emotion-replies": {"msg-ids"},
		"add-text-emotion":     {"background-id", "conversation-id", "emotion-id", "emotion-name", "message-id", "text"},
		"remove-text-emotion":  {"background-id", "conversation-id", "emotion-id", "emotion-name", "message-id", "text"},
		"update-text-emotion":  {"background-id", "conversation-id", "emotion-id", "emotion-name", "message-id", "old-emotion-id", "text"},
	}
	for leaf, want := range wantByLeaf {
		command, remaining, err := root.Find([]string{"thread", leaf})
		if err != nil || len(remaining) != 0 {
			t.Fatalf("find chat thread %s: command=%v remaining=%v error=%v", leaf, command, remaining, err)
		}
		got := make([]string, 0, len(want))
		command.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
			if flag.Name != "help" && !flag.Hidden {
				got = append(got, flag.Name)
			}
		})
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("chat thread %s public flags = %v, want pre-split flags %v", leaf, got, want)
		}
	}
}

func TestCrossPlatformCoverageAtomicThreadReplyUsesDirectThreadTarget(t *testing.T) {
	caller := &chatThreadCaller{}
	err := executeAtomicThreadCommand(t, caller,
		"thread", "reply", "--conversation-id", "thread-1", "--text", "收到")
	if err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("calls = %#v, want one write", caller.calls)
	}
	call := caller.calls[0]
	if call.product != "" || call.tool != "send_personal_message" || call.args["openConversationId"] != "thread-1" {
		t.Fatalf("reply call = %#v", call)
	}
	if call.args["referenceOpenMessageId"] != nil || call.args["quotedMessage"] != nil {
		t.Fatalf("thread reply carried quote fields: %#v", call.args)
	}
}

func TestCrossPlatformCoverageAtomicThreadCompatibilityMappings(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		responses := map[string]string{
			"contact/get_current_user_profile": `{"result":{"userId":"owner-1"}}`,
			"im/create_group_conversation":     `{"result":{"openCid":"topic-1"}}`,
		}
		legacy := &chatThreadCaller{responses: responses}
		if err := executeAtomicThreadCommand(t, legacy,
			"group", "create", "--name", "话题圈", "--users", "user-1", "--thread"); err != nil {
			t.Fatal(err)
		}
		thread := &chatThreadCaller{responses: responses}
		if err := executeAtomicThreadCommand(t, thread,
			"thread", "create-group", "--name", "话题圈", "--users", "user-1"); err != nil {
			t.Fatal(err)
		}
		if len(legacy.calls) != 2 || len(thread.calls) != 2 ||
			legacy.calls[1].product != thread.calls[1].product || legacy.calls[1].tool != thread.calls[1].tool ||
			!reflect.DeepEqual(legacy.calls[1].args, thread.calls[1].args) {
			t.Fatalf("legacy=%#v thread=%#v", legacy.calls, thread.calls)
		}
	})

	t.Run("send", func(t *testing.T) {
		legacy := &chatThreadCaller{}
		if err := executeAtomicThreadCommand(t, legacy,
			"message", "send", "--conversation-id", "topic-1", "--text", "新话题", "--idempotency-key", "send-1"); err != nil {
			t.Fatal(err)
		}
		thread := &chatThreadCaller{}
		if err := executeAtomicThreadCommand(t, thread,
			"thread", "send", "--conversation-id", "topic-1", "--text", "新话题", "--idempotency-key", "send-1"); err != nil {
			t.Fatal(err)
		}
		if len(legacy.calls) != 1 || len(thread.calls) != 1 ||
			legacy.calls[0].product != thread.calls[0].product || legacy.calls[0].tool != thread.calls[0].tool ||
			!reflect.DeepEqual(legacy.calls[0].args, thread.calls[0].args) {
			t.Fatalf("legacy=%#v thread=%#v", legacy.calls, thread.calls)
		}
	})

	for _, test := range []struct {
		name       string
		threadPath string
		targetFlag string
		target     string
	}{
		{name: "send mentions", threadPath: "send", targetFlag: "--conversation-id", target: "topic-1"},
		{name: "reply mentions", threadPath: "reply", targetFlag: "--conversation-id", target: "thread-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			legacy := &chatThreadCaller{}
			if err := executeAtomicThreadCommand(t, legacy,
				"message", "send", "--conversation-id", test.target, "--content", "通知 <@user-open-id> <@all>",
				"--at-open-dingtalk-ids", "user-open-id", "--at-all"); err != nil {
				t.Fatal(err)
			}
			thread := &chatThreadCaller{}
			if err := executeAtomicThreadCommand(t, thread,
				"thread", test.threadPath, test.targetFlag, test.target, "--content", "通知 <@user-open-id> <@all>",
				"--at-open-dingtalk-ids", "user-open-id", "--at-all"); err != nil {
				t.Fatal(err)
			}
			if len(legacy.calls) != 1 || len(thread.calls) != 1 ||
				legacy.calls[0].product != thread.calls[0].product || legacy.calls[0].tool != thread.calls[0].tool ||
				!reflect.DeepEqual(legacy.calls[0].args, thread.calls[0].args) {
				t.Fatalf("legacy=%#v thread=%#v", legacy.calls, thread.calls)
			}
		})
	}

	t.Run("list", func(t *testing.T) {
		responses := map[string]string{
			"chat/list_conversation_message_v2": `{"result":{"messages":[],"hasMore":false}}`,
		}
		legacy := &chatThreadCaller{responses: responses}
		if err := executeAtomicThreadCommand(t, legacy,
			"message", "list", "--conversation-id", "topic-1", "--time", "2026-08-18 10:00:00", "--direction", "older", "--limit", "20"); err != nil {
			t.Fatal(err)
		}
		thread := &chatThreadCaller{responses: responses}
		if err := executeAtomicThreadCommand(t, thread,
			"thread", "list", "--conversation-id", "topic-1", "--time", "2026-08-18 10:00:00", "--direction", "older", "--limit", "20"); err != nil {
			t.Fatal(err)
		}
		if len(legacy.calls) != 1 || len(thread.calls) != 1 ||
			legacy.calls[0].product != thread.calls[0].product || legacy.calls[0].tool != thread.calls[0].tool ||
			!reflect.DeepEqual(legacy.calls[0].args, thread.calls[0].args) {
			t.Fatalf("legacy=%#v thread=%#v", legacy.calls, thread.calls)
		}
	})

	t.Run("list default time", func(t *testing.T) {
		responses := map[string]string{
			"chat/list_conversation_message_v2": `{"result":{"messages":[],"hasMore":false}}`,
		}
		legacy := &chatThreadCaller{responses: responses}
		if err := executeAtomicThreadCommand(t, legacy,
			"message", "list", "--conversation-id", "topic-1"); err != nil {
			t.Fatal(err)
		}
		thread := &chatThreadCaller{responses: responses}
		if err := executeAtomicThreadCommand(t, thread,
			"thread", "list", "--conversation-id", "topic-1"); err != nil {
			t.Fatal(err)
		}
		if len(legacy.calls) != 1 || len(thread.calls) != 1 {
			t.Fatalf("legacy=%#v thread=%#v", legacy.calls, thread.calls)
		}
		legacyTime, legacyErr := parseISOTimeToMillis("time", legacy.calls[0].args["time"].(string))
		threadTime, threadErr := parseISOTimeToMillis("time", thread.calls[0].args["time"].(string))
		if legacyErr != nil || threadErr != nil || legacyTime-threadTime > 5000 || threadTime-legacyTime > 5000 {
			t.Fatalf("default times differ: legacy=%#v thread=%#v errors=(%v, %v)", legacy.calls[0].args["time"], thread.calls[0].args["time"], legacyErr, threadErr)
		}
		legacy.calls[0].args["time"] = "<default-time>"
		thread.calls[0].args["time"] = "<default-time>"
		if legacy.calls[0].product != thread.calls[0].product || legacy.calls[0].tool != thread.calls[0].tool ||
			!reflect.DeepEqual(legacy.calls[0].args, thread.calls[0].args) {
			t.Fatalf("legacy=%#v thread=%#v", legacy.calls, thread.calls)
		}
	})

	t.Run("reply", func(t *testing.T) {
		legacy := &chatThreadCaller{}
		if err := executeAtomicThreadCommand(t, legacy,
			"message", "send", "--conversation-id", "thread-1", "--text", "直接回复", "--idempotency-key", "reply-1"); err != nil {
			t.Fatal(err)
		}
		thread := &chatThreadCaller{}
		if err := executeAtomicThreadCommand(t, thread,
			"thread", "reply", "--conversation-id", "thread-1", "--text", "直接回复", "--idempotency-key", "reply-1"); err != nil {
			t.Fatal(err)
		}
		if len(legacy.calls) != 1 || len(thread.calls) != 1 ||
			legacy.calls[0].product != thread.calls[0].product || legacy.calls[0].tool != thread.calls[0].tool ||
			!reflect.DeepEqual(legacy.calls[0].args, thread.calls[0].args) {
			t.Fatalf("legacy=%#v thread=%#v", legacy.calls, thread.calls)
		}
	})

	t.Run("reply preuploaded file", func(t *testing.T) {
		legacy := &chatThreadCaller{}
		if err := executeAtomicThreadCommand(t, legacy,
			"message", "send", "--conversation-id", "thread-1", "--msg-type", "file",
			"--dentry-id", "101", "--space-id", "202", "--file-name", "fixture.txt",
			"--file-type", "txt", "--file-size", "12", "--file", "/fixture.txt"); err != nil {
			t.Fatal(err)
		}
		thread := &chatThreadCaller{}
		if err := executeAtomicThreadCommand(t, thread,
			"thread", "reply", "--conversation-id", "thread-1", "--msg-type", "file",
			"--dentry-id", "101", "--space-id", "202", "--file-name", "fixture.txt",
			"--file-type", "txt", "--file-size", "12", "--file", "/fixture.txt"); err != nil {
			t.Fatal(err)
		}
		if len(legacy.calls) != 1 || len(thread.calls) != 1 ||
			legacy.calls[0].product != thread.calls[0].product || legacy.calls[0].tool != thread.calls[0].tool ||
			!reflect.DeepEqual(legacy.calls[0].args, thread.calls[0].args) {
			t.Fatalf("legacy=%#v thread=%#v", legacy.calls, thread.calls)
		}
	})

	t.Run("list replies", func(t *testing.T) {
		caller := &chatThreadCaller{responses: map[string]string{
			"chat/list_topic_replies": `{"result":{"messages":[],"hasMore":false}}`,
		}}
		err := executeAtomicThreadCommand(t, caller,
			"thread", "list-replies", "--conversation-id", "topic-1", "--topic-id", "thread-1", "--time", "2026-08-18 10:00:00", "--direction", "newer", "--limit", "20")
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"openconversationId": "topic-1", "topicId": "thread-1", "startTime": "2026-08-18 10:00:00", "forward": true, "pageSize": 20}
		if len(caller.calls) != 1 || caller.calls[0].product != "chat" || caller.calls[0].tool != "list_topic_replies" || !reflect.DeepEqual(caller.calls[0].args, want) {
			t.Fatalf("calls = %#v, want %#v", caller.calls, want)
		}
		legacy := &chatThreadCaller{responses: map[string]string{
			"chat/list_topic_replies": `{"result":{"messages":[],"hasMore":false}}`,
		}}
		if err := executeAtomicThreadCommand(t, legacy,
			"message", "list-topic-replies", "--conversation-id", "topic-1", "--topic-id", "thread-1", "--time", "2026-08-18 10:00:00", "--direction", "newer", "--limit", "20"); err != nil {
			t.Fatal(err)
		}
		if len(legacy.calls) != 1 || !reflect.DeepEqual(legacy.calls[0].args, caller.calls[0].args) {
			t.Fatalf("legacy=%#v thread=%#v", legacy.calls, caller.calls)
		}
	})

	t.Run("forward", func(t *testing.T) {
		caller := &chatThreadCaller{}
		err := executeAtomicThreadCommand(t, caller,
			"thread", "forward", "--src-msg-id", "message-1", "--src-conversation-id", "topic-1", "--src-thread-id", "thread-1", "--dest-conversation-id", "conversation-2")
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]any{
			"srcOpenMessageId": "message-1", "srcOpenConversationId": "topic-1",
			"srcOpenConvThreadId": "thread-1", "destOpenConversationId": "conversation-2",
		}
		if len(caller.calls) != 1 || caller.calls[0].product != "im" || caller.calls[0].tool != "forward_topic" || !reflect.DeepEqual(caller.calls[0].args, want) {
			t.Fatalf("calls = %#v, want %#v", caller.calls, want)
		}
		legacy := &chatThreadCaller{}
		if err := executeAtomicThreadCommand(t, legacy,
			"message", "forward-topic", "--src-msg-id", "message-1", "--src-conversation-id", "topic-1", "--src-thread-id", "thread-1", "--dest-conversation-id", "conversation-2"); err != nil {
			t.Fatal(err)
		}
		if len(legacy.calls) != 1 || !reflect.DeepEqual(legacy.calls[0].args, caller.calls[0].args) {
			t.Fatalf("legacy=%#v thread=%#v", legacy.calls, caller.calls)
		}
	})
}

func TestCrossPlatformCoverageAtomicThreadQuoteReplyIsRejectedBeforeWrite(t *testing.T) {
	caller := &chatThreadCaller{responses: map[string]string{
		"im/list_messages_by_ids": `{"result":{"messages":[{"openMessageId":"root-1","openConvThreadId":"thread-1"}]}}`,
	}}
	err := executeAtomicThreadCommand(t, caller,
		"message", "reply", "--conversation-id", "topic-1", "--ref-msg-id", "root-1",
		"--ref-sender", "DAAAAAAAAAAAiE", "--text", "错误引用")
	var typed *apperrors.Error
	if err == nil || !errors.As(err, &typed) || typed.Reason != "topic_quote_reply_disabled" {
		t.Fatalf("error = %v", err)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != "list_messages_by_ids" {
		t.Fatalf("quote guard reached write: %#v", caller.calls)
	}
}

func TestCrossPlatformCoverageAtomicThreadBotQuoteReplyIsRejectedBeforeWrite(t *testing.T) {
	caller := &chatThreadCaller{responses: map[string]string{
		"im/list_messages_by_ids": `{"result":{"messages":[{"openMessageId":"root-1","openConvThreadId":"thread-1"}]}}`,
	}}
	err := executeAtomicThreadCommand(t, caller,
		"message", "send-by-bot", "--robot-code", "robot-1", "--conversation-id", "topic-1",
		"--reply", "root-1", "--ref-sender", "DAAAAAAAAAAAiE", "--text", "错误引用")
	var typed *apperrors.Error
	if err == nil || !errors.As(err, &typed) || typed.Reason != "topic_quote_reply_disabled" {
		t.Fatalf("error = %v", err)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != "list_messages_by_ids" {
		t.Fatalf("bot quote guard reached write: %#v", caller.calls)
	}
}

func TestCrossPlatformCoverageAtomicThreadQuoteReplyDryRunSkipsRemotePreflight(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{
			name: "message reply",
			args: []string{
				"message", "reply", "--conversation-id", "conversation-1", "--ref-msg-id", "message-1",
				"--ref-sender", "DAAAAAAAAAAAiE", "--text", "dry-run reply",
			},
		},
		{
			name: "message send-by-bot reply",
			args: []string{
				"message", "send-by-bot", "--robot-code", "robot-1", "--conversation-id", "conversation-1",
				"--reply", "message-1", "--ref-sender", "DAAAAAAAAAAAiE", "--text", "dry-run reply",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			testseam.Protect(t, &deps)
			caller := &chatThreadCaller{dryRun: true}
			if err := runChatCoverageCommand(t, caller, test.args...); err != nil {
				t.Fatal(err)
			}
			if len(caller.calls) != 0 {
				t.Fatalf("dry-run made remote calls: %#v", caller.calls)
			}
		})
	}
}

func TestCrossPlatformCoverageAtomicThreadQuoteGuardFailsClosedWhenConversationLookupFails(t *testing.T) {
	caller := &chatThreadCaller{
		responses: map[string]string{
			"im/list_messages_by_ids": `{"result":{"messages":[{"openMessageId":"root-1","openConversationId":"topic-1"}]}}`,
		},
		errors: map[string]error{"chat/get_conversation_info": errors.New("conversation lookup unavailable")},
	}
	err := executeAtomicThreadCommand(t, caller,
		"message", "reply", "--conversation-id", "topic-1", "--ref-msg-id", "root-1",
		"--ref-sender", "DAAAAAAAAAAAiE", "--text", "错误引用")
	var typed *apperrors.Error
	if err == nil || !errors.As(err, &typed) || typed.Reason != "topic_quote_guard_unavailable" {
		t.Fatalf("error = %v", err)
	}
	if len(caller.calls) != 2 || caller.calls[1].tool != "get_conversation_info" {
		t.Fatalf("quote guard reached write: %#v", caller.calls)
	}
}

func TestCrossPlatformCoverageAtomicThreadQuoteGuardFailsClosedWhenMessageLookupFails(t *testing.T) {
	caller := &chatThreadCaller{errors: map[string]error{
		"im/list_messages_by_ids": errors.New("message lookup unavailable"),
	}}
	err := executeAtomicThreadCommand(t, caller,
		"message", "reply", "--conversation-id", "topic-1", "--ref-msg-id", "root-1",
		"--ref-sender", "DAAAAAAAAAAAiE", "--text", "错误引用")
	var typed *apperrors.Error
	if err == nil || !errors.As(err, &typed) || typed.Reason != "topic_quote_guard_unavailable" {
		t.Fatalf("error = %v", err)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != "list_messages_by_ids" {
		t.Fatalf("quote guard reached write: %#v", caller.calls)
	}
}

func TestCrossPlatformCoverageAtomicThreadQuoteGuardRejectsConversationFailuresAndTopics(t *testing.T) {
	for _, test := range []struct {
		name       string
		response   string
		wantReason string
	}{
		{name: "invalid response", response: `<html>bad gateway</html>`, wantReason: "topic_quote_guard_unavailable"},
		{name: "topic conversation", response: `{"result":{"convThreadEnabled":true}}`, wantReason: "topic_quote_reply_disabled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := &chatThreadCaller{responses: map[string]string{
				"im/list_messages_by_ids":    `{"result":{"messages":[{"openMessageId":"root-1","openConversationId":"topic-1"}]}}`,
				"chat/get_conversation_info": test.response,
			}}
			err := executeAtomicThreadCommand(t, caller,
				"message", "reply", "--conversation-id", "topic-1", "--ref-msg-id", "root-1",
				"--ref-sender", "DAAAAAAAAAAAiE", "--text", "错误引用")
			var typed *apperrors.Error
			if err == nil || !errors.As(err, &typed) || typed.Reason != test.wantReason {
				t.Fatalf("error = %v, want reason %q", err, test.wantReason)
			}
			if len(caller.calls) != 2 || caller.calls[1].tool != "get_conversation_info" {
				t.Fatalf("quote guard calls = %#v", caller.calls)
			}
		})
	}
}

func TestCrossPlatformCoverageAtomicThreadQuoteGuardFailsClosedWithoutConversationState(t *testing.T) {
	caller := &chatThreadCaller{responses: map[string]string{
		"im/list_messages_by_ids":    `{"result":{"messages":[{"openMessageId":"message-1","openConversationId":"cid"}]}}`,
		"chat/get_conversation_info": `{"result":{"openConversationId":"cid"}}`,
	}}
	err := executeAtomicThreadCommand(t, caller,
		"message", "reply", "--conversation-id", "cid", "--ref-msg-id", "message-1",
		"--ref-sender", "DAAAAAAAAAAAiE", "--text", "普通引用")
	var typed *apperrors.Error
	if err == nil || !errors.As(err, &typed) || typed.Reason != "topic_quote_guard_unavailable" {
		t.Fatalf("error = %v", err)
	}
	if len(caller.calls) != 2 || caller.calls[1].tool != "get_conversation_info" {
		t.Fatalf("quote guard reached write: %#v", caller.calls)
	}
}

func TestCrossPlatformCoverageChatThreadMessageCommandsValidateAndReuseExistingPayloads(t *testing.T) {
	messageResponse := `{"result":{"messages":[{"openMessageId":"message-1","openConvThreadId":"thread-1"}]}}`
	for _, test := range []struct {
		name     string
		args     []string
		wantTool string
		wantArgs map[string]any
	}{
		{
			name:     "recall message",
			args:     []string{"thread", "recall-message", "--conversation-id", "conversation-1", "--message-id", "message-1"},
			wantTool: "recall_message",
			wantArgs: map[string]any{"openConversationId": "conversation-1", "openMessageId": "message-1"},
		},
		{
			name:     "add emoji",
			args:     []string{"thread", "add-emoji", "--conversation-id", "conversation-1", "--message-id", "message-1", "--emoji", "赞"},
			wantTool: "add_emoji_reaction",
			wantArgs: map[string]any{"openConversationId": "conversation-1", "openMsgId": "message-1", "emojiName": "赞"},
		},
		{
			name:     "remove emoji",
			args:     []string{"thread", "remove-emoji", "--conversation-id", "conversation-1", "--message-id", "message-1", "--emoji", "赞"},
			wantTool: "remove_emoji_reaction",
			wantArgs: map[string]any{"openConversationId": "conversation-1", "openMsgId": "message-1", "emojiName": "赞"},
		},
		{
			name:     "add text emotion",
			args:     []string{"thread", "add-text-emotion", "--conversation-id", "conversation-1", "--message-id", "message-1", "--emotion-id", "emotion-1", "--emotion-name", "处理中", "--text", "处理中", "--background-id", "bg-1"},
			wantTool: "add_text_emotion",
			wantArgs: map[string]any{"openConversationId": "conversation-1", "openMsgId": "message-1", "emotionId": "emotion-1", "emotionName": "处理中", "text": "处理中", "backgroundId": "bg-1"},
		},
		{
			name:     "remove text emotion",
			args:     []string{"thread", "remove-text-emotion", "--conversation-id", "conversation-1", "--message-id", "message-1", "--emotion-id", "emotion-1", "--emotion-name", "处理中", "--text", "处理中", "--background-id", "bg-1"},
			wantTool: "remove_text_emotion",
			wantArgs: map[string]any{"openConversationId": "conversation-1", "openMsgId": "message-1", "emotionId": "emotion-1", "emotionName": "处理中", "text": "处理中", "backgroundId": "bg-1"},
		},
		{
			name:     "update text emotion",
			args:     []string{"thread", "update-text-emotion", "--conversation-id", "conversation-1", "--message-id", "message-1", "--old-emotion-id", "emotion-old", "--emotion-id", "emotion-new", "--emotion-name", "已完成", "--text", "已完成", "--background-id", "bg-2"},
			wantTool: "update_text_emotion",
			wantArgs: map[string]any{"openConversationId": "conversation-1", "openMsgId": "message-1", "oldEmotionId": "emotion-old", "emotionId": "emotion-new", "emotionName": "已完成", "text": "已完成", "backgroundId": "bg-2"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := &chatThreadCaller{responses: map[string]string{"im/list_messages_by_ids": messageResponse}}
			if err := executeAtomicThreadCommand(t, caller, test.args...); err != nil {
				t.Fatal(err)
			}
			if len(caller.calls) != 2 || caller.calls[0].tool != "list_messages_by_ids" || caller.calls[1].tool != test.wantTool || !reflect.DeepEqual(caller.calls[1].args, test.wantArgs) {
				t.Fatalf("calls = %#v, want tool=%s args=%#v", caller.calls, test.wantTool, test.wantArgs)
			}
		})
	}
}

func TestCrossPlatformCoverageChatThreadReplyMessageKeepsParentConversationSemantics(t *testing.T) {
	caller := &chatThreadCaller{responses: map[string]string{
		"im/list_messages_by_ids":           `{"result":{"messages":[{"openMessageId":"reply-1","openConversationId":"parent-1"}]}}`,
		"chat/list_conversation_message_v2": `{"result":{"messages":[{"openMessageId":"root-1","openConvThreadId":"thread-1"}],"hasMore":false}}`,
		"chat/list_topic_replies":           `{"result":{"messages":[{"openMessageId":"reply-1","openConversationId":"parent-1"}],"hasMore":false}}`,
	}}
	if err := executeAtomicThreadCommand(t, caller,
		"thread", "recall-message", "--conversation-id", "parent-1", "--message-id", "reply-1"); err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 4 || caller.calls[0].tool != "list_messages_by_ids" ||
		caller.calls[1].tool != "list_conversation_message_v2" || caller.calls[2].tool != "list_topic_replies" ||
		caller.calls[3].tool != "recall_message" {
		t.Fatalf("calls = %#v", caller.calls)
	}
	wantRecall := map[string]any{"openConversationId": "parent-1", "openMessageId": "reply-1"}
	if !reflect.DeepEqual(caller.calls[3].args, wantRecall) {
		t.Fatalf("recall args = %#v, want %#v", caller.calls[3].args, wantRecall)
	}
	wantLookup := map[string]any{
		"openconversationId": "parent-1",
		"topicId":            "thread-1",
		"forward":            false,
		"pageSize":           100,
	}
	if !reflect.DeepEqual(caller.calls[2].args, wantLookup) {
		t.Fatalf("reply lookup args = %#v, want %#v", caller.calls[2].args, wantLookup)
	}
}

func TestCrossPlatformCoverageChatThreadBatchEmotionAndOwnershipFailures(t *testing.T) {
	caller := &chatThreadCaller{responses: map[string]string{
		"im/list_messages_by_ids": `{"result":{"messages":[{"openMessageId":"message-1","openConvThreadId":"thread-1"},{"openMessageId":"message-2","openConvThreadId":"thread-1"}]}}`,
	}}
	if err := executeAtomicThreadCommand(t, caller, "thread", "list-emotion-replies", "--msg-ids", "message-1,message-2"); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"openMessageIds": []string{"message-1", "message-2"}}
	if len(caller.calls) != 2 || caller.calls[1].tool != "list_message_emotion_replies" || !reflect.DeepEqual(caller.calls[1].args, want) {
		t.Fatalf("calls = %#v, want %#v", caller.calls, want)
	}

	nonThread := &chatThreadCaller{responses: map[string]string{
		"im/list_messages_by_ids": `{"result":{"messages":[{"openMessageId":"message-1"}]}}`,
	}}
	err := executeAtomicThreadCommand(t, nonThread, "thread", "add-emoji", "--conversation-id", "conversation-1", "--message-id", "message-1", "--emoji", "赞")
	var typed *apperrors.Error
	if err == nil || !errors.As(err, &typed) || typed.Reason != "message_not_in_thread" || len(nonThread.calls) != 1 {
		t.Fatalf("error=%v calls=%#v", err, nonThread.calls)
	}
}

func TestCrossPlatformCoverageDetectTopicContainerState(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		want  topicContainerState
	}{
		{name: "bool true", value: map[string]any{"convThreadEnabled": true}, want: topicContainerTopic},
		{name: "bool false", value: map[string]any{"convThreadEnabled": false}, want: topicContainerNonTopic},
		{name: "string true", value: map[string]any{"convThreadEnabled": "true"}, want: topicContainerTopic},
		{name: "string false", value: map[string]any{"convThreadEnabled": "0"}, want: topicContainerNonTopic},
		{name: "invalid string", value: map[string]any{"convThreadEnabled": "unknown"}, want: topicContainerUnknown},
		{name: "invalid type", value: map[string]any{"convThreadEnabled": 1}, want: topicContainerUnknown},
		{name: "nested map", value: map[string]any{"result": map[string]any{"convThreadEnabled": true}}, want: topicContainerTopic},
		{name: "nested array", value: []any{map[string]any{"convThreadEnabled": true}}, want: topicContainerTopic},
		{name: "topic group", value: map[string]any{"topicGroup": "1"}, want: topicContainerTopic},
		{name: "is topic group false", value: map[string]any{"isTopicGroup": false}, want: topicContainerNonTopic},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := detectTopicContainerState(test.value); got != test.want {
				t.Fatalf("state = %v, want %v", got, test.want)
			}
		})
	}
}
