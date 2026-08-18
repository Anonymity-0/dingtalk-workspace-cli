// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package mail

import (
	"fmt"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func mailResolveMailbox(rt *shortcut.RuntimeContext) (string, error) {
	if email := strings.TrimSpace(rt.Str("email")); email != "" {
		return email, nil
	}
	data, err := rt.CallMCPData("mail", "list_user_mailboxes", nil)
	if err != nil {
		return "", err
	}
	mailboxes, err := mailRequireCollection(data, "mail/list_user_mailboxes", "emailAccounts")
	if err != nil {
		return "", err
	}
	if len(mailboxes) == 0 {
		return "", apperrors.NewValidation("当前身份没有可用邮箱，请用 --email 指定")
	}
	email := mailFirstString(mailboxes[0], "email")
	if email == "" {
		return "", mailResponseError("mail/list_user_mailboxes", "missing_mailbox_identity", "首个邮箱对象缺少 email")
	}
	return email, nil
}

func mailReadMessage(rt *shortcut.RuntimeContext, email, id string) (map[string]any, error) {
	data, err := rt.CallMCPData("mail", "get_email_by_message_id", map[string]any{"email": email, "messageId": id})
	if err != nil {
		return nil, err
	}
	message, err := mailRequireObject(data, "mail/get_email_by_message_id", "message")
	if err != nil {
		return nil, err
	}
	if err := mailRequireIdentity(message, "mail/get_email_by_message_id", id, "id"); err != nil {
		return nil, err
	}
	return message, nil
}

var Message = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "mail", Command: "+message", Product: "mail",
	Description: "读取一封邮件的完整正文与附件元数据", Intent: "已知单个 messageId，需要读取完整正文和附件元数据时使用；自动解析当前邮箱，并要求返回邮件 ID 与请求精确一致。",
	Risk: shortcut.RiskRead, Safety: mailReadSafety(),
	Contract: mailReadContract("+message", "读取一封邮件的完整正文与附件元数据", "已知单个 messageId，需要读取完整正文和附件元数据时使用；自动解析当前邮箱，并要求返回邮件 ID 与请求精确一致。", mailObjectResult("身份匹配的单封邮件详情"), []contract.ParamDecl{{Name: "id", Property: "messageId"}}, `dws mail +message --id <messageId> --format json`),
	Flags: []shortcut.Flag{
		{Name: "id", Type: shortcut.FlagString, Required: true, Desc: "邮件 messageId，不能为空"},
		{Name: "email", Type: shortcut.FlagString, Desc: "邮箱地址；不传时自动取当前身份首个邮箱"},
	},
	Constraints: []shortcut.Constraint{{Kind: shortcut.ConstraintCustom, Flags: []string{"id"}, Description: "不能为空"}},
	Validate: func(rt *shortcut.RuntimeContext) error {
		return mailValidateRequiredText(rt, "id")
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		email, err := mailResolveMailbox(rt)
		if err != nil {
			return err
		}
		message, err := mailReadMessage(rt, email, rt.Str("id"))
		if err != nil {
			return err
		}
		return rt.Output(map[string]any{"value": message})
	},
}

var Messages = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "mail", Command: "+messages", Product: "mail",
	Description: "按请求顺序读取多封邮件并逐封验证身份", Intent: "需要一次读取多个 messageId 时使用；按输入顺序逐封读取，任何缺失、错型或身份不匹配都会使整次任务失败。",
	Risk: shortcut.RiskRead, Safety: mailReadSafety(),
	Contract: mailReadContract("+messages", "按请求顺序读取多封邮件并逐封验证身份", "需要一次读取多个 messageId 时使用；按输入顺序逐封读取，任何缺失、错型或身份不匹配都会使整次任务失败。", mailCollectionResult("messages", "身份匹配且保持请求顺序的邮件详情"), []contract.ParamDecl{{Name: "ids", Property: "messageIds"}}, `dws mail +messages --ids <id1>,<id2> --format json`),
	Flags: []shortcut.Flag{
		{Name: "ids", Type: shortcut.FlagStringSlice, Required: true, Desc: "邮件 messageId 列表，1-100 个且每项不能为空"},
		{Name: "email", Type: shortcut.FlagString, Desc: "邮箱地址；不传时自动取当前身份首个邮箱"},
	},
	Constraints: []shortcut.Constraint{{
		Kind: shortcut.ConstraintCustom, Flags: []string{"ids"},
		Description: "1-100 个且不能为空",
	}},
	Validate: func(rt *shortcut.RuntimeContext) error {
		return mailValidateMessageIDs(rt.StrSlice("ids"))
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		ids := rt.StrSlice("ids")
		email, err := mailResolveMailbox(rt)
		if err != nil {
			return err
		}
		messages := make([]map[string]any, 0, len(ids))
		for index, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				return apperrors.NewValidation(fmt.Sprintf("--ids 第 %d 项为空", index))
			}
			message, err := mailReadMessage(rt, email, id)
			if err != nil {
				return err
			}
			messages = append(messages, message)
		}
		return rt.Output(mailBusinessCollectionPayload("messages", messages))
	},
}

func mailValidateMessageIDs(ids []string) error {
	if len(ids) == 0 || len(ids) > 100 {
		return apperrors.NewValidation("--ids 需要 1 到 100 个邮件 ID")
	}
	for index, id := range ids {
		if strings.TrimSpace(id) == "" {
			return apperrors.NewValidation(fmt.Sprintf("--ids 第 %d 项为空", index))
		}
	}
	return nil
}

var Thread = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "mail", Command: "+thread", Product: "mail",
	Description: "读取完整邮件会话并精确验证 conversationId", Intent: "已知一个 conversationId，需要查看同一主题的会话上下文时使用；自动解析邮箱并拒绝空对象或错会话。",
	Risk: shortcut.RiskRead, Safety: mailReadSafety(),
	Contract: mailReadContract("+thread", "读取完整邮件会话并精确验证 conversationId", "已知一个 conversationId，需要查看同一主题的会话上下文时使用；自动解析邮箱并拒绝空对象或错会话。", mailObjectResult("身份匹配的邮件会话详情"), []contract.ParamDecl{{Name: "id", Property: "conversationId"}}, `dws mail +thread --id <conversationId> --format json`),
	Flags: []shortcut.Flag{
		{Name: "id", Type: shortcut.FlagString, Required: true, Desc: "邮件会话 conversationId，不能为空"},
		{Name: "email", Type: shortcut.FlagString, Desc: "邮箱地址；不传时自动取当前身份首个邮箱"},
	},
	Constraints: []shortcut.Constraint{{Kind: shortcut.ConstraintCustom, Flags: []string{"id"}, Description: "不能为空"}},
	Validate: func(rt *shortcut.RuntimeContext) error {
		return mailValidateRequiredText(rt, "id")
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		email, err := mailResolveMailbox(rt)
		if err != nil {
			return err
		}
		data, err := rt.CallMCPData("mail", "get_thread", map[string]any{"email": email, "conversationId": rt.Str("id")})
		if err != nil {
			return err
		}
		conversation, err := mailRequireObject(data, "mail/get_thread", "conversation")
		if err != nil {
			return err
		}
		if err := mailRequireIdentity(conversation, "mail/get_thread", rt.Str("id"), "id"); err != nil {
			return err
		}
		return rt.Output(map[string]any{"value": conversation})
	},
}

func init() {
	shortcut.Register(Message, Messages, Thread)
}
