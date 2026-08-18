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

func mailRecipients(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}

func mailDraftMessage(data map[string]any, operation string) (map[string]any, error) {
	if err := mailRequireSuccess(data, operation); err != nil {
		return nil, err
	}
	value, present := mailLookup(data, "result.message")
	if !present {
		return nil, mailResponseError(operation, "missing_write_receipt", "写响应缺少 result.message")
	}
	message, ok := value.(map[string]any)
	if !ok || len(message) == 0 {
		return nil, mailResponseError(operation, "malformed_write_receipt", fmt.Sprintf("result.message 应为非空对象，实际为 %T", value))
	}
	if err := mailRequireIdentity(message, operation, "", "id"); err != nil {
		return nil, err
	}
	return message, nil
}

func mailReadTemplate(rt *shortcut.RuntimeContext, email, id string) (map[string]any, error) {
	data, err := rt.CallMCPData("mail", "get_user_message_template", map[string]any{"email": email, "id": id})
	if err != nil {
		return nil, err
	}
	if err := mailRequireSuccess(data, "mail/get_user_message_template"); err != nil {
		return nil, err
	}
	template := make(map[string]any, len(data))
	for key, value := range data {
		switch key {
		case "success", "errorCode", "errorMsg", "arguments":
			continue
		default:
			template[key] = value
		}
	}
	if len(template) == 0 {
		return nil, mailResponseError("mail/get_user_message_template", "missing_result", "模板详情为空")
	}
	if err := mailRequireIdentity(template, "mail/get_user_message_template", id, "id"); err != nil {
		return nil, err
	}
	return template, nil
}

func mailTemplateSubject(template map[string]any) string {
	message, _ := template["message"].(map[string]any)
	return mailFirstString(message, "subject")
}

func mailTemplateBody(template map[string]any) string {
	message, _ := template["message"].(map[string]any)
	return mailFirstString(message, "markdownBody", "body")
}

var DraftCreate = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "mail", Command: "+draft-create", Product: "mail",
	Description: "创建邮件草稿并按 messageId 读回验证", Intent: "撰写一封新邮件但暂不发送；取得稳定草稿 ID 后读取同一对象，并核对主题和正文。",
	Risk: shortcut.RiskWrite, Safety: mailWriteSafety("non_idempotent"),
	Contract: mailWriteContract("+draft-create", "创建邮件草稿并按 messageId 读回验证", "撰写一封新邮件但暂不发送；取得稳定草稿 ID 后读取同一对象，并核对主题和正文。", []contract.ParamDecl{{Name: "from", Property: "from"}, {Name: "to", Property: "toRecipients"}, {Name: "cc", Property: "ccRecipients"}, {Name: "body", Property: "body"}}, `dws mail +draft-create --from user@company.com --subject "草稿" --body "正文" --format json`),
	Flags: []shortcut.Flag{
		{Name: "from", Type: shortcut.FlagString, Required: true, Desc: "发件邮箱"},
		{Name: "to", Type: shortcut.FlagStringSlice, Desc: "收件邮箱，可多次指定或逗号分隔"},
		{Name: "cc", Type: shortcut.FlagStringSlice, Desc: "抄送邮箱，可多次指定或逗号分隔"},
		{Name: "subject", Type: shortcut.FlagString, Required: true, Desc: "草稿主题"},
		{Name: "body", Type: shortcut.FlagString, Desc: "草稿正文"},
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{"from": rt.Str("from"), "subject": rt.Str("subject")}
		if recipients := mailRecipients(rt.StrSlice("to")); len(recipients) > 0 {
			params["toRecipients"] = recipients
		}
		if recipients := mailRecipients(rt.StrSlice("cc")); len(recipients) > 0 {
			params["ccRecipients"] = recipients
		}
		if rt.Changed("body") {
			params["body"] = rt.Str("body")
		}
		if rt.DryRun() {
			return rt.Output(map[string]any{"value": map[string]any{"dryRun": true, "executed": false, "operation": "mail/create_draft"}})
		}
		written, err := rt.CallMCPWriteDataStrict("mail", "create_draft", params)
		if err != nil {
			return err
		}
		receipt, err := mailDraftMessage(written, "mail/create_draft")
		if err != nil {
			return err
		}
		id := mailFirstString(receipt, "id")
		verified, err := mailReadMessage(rt, rt.Str("from"), id)
		if err != nil {
			return err
		}
		if mailFirstString(verified, "subject") != rt.Str("subject") {
			return mailResponseError("mail/create_draft", "verification_mismatch", "草稿读回主题与请求不一致")
		}
		if rt.Changed("body") && mailFirstString(verified, "markdownBody", "body") != rt.Str("body") {
			return mailResponseError("mail/create_draft", "verification_mismatch", "草稿读回正文与请求不一致")
		}
		return rt.Output(map[string]any{"value": verified})
	},
}

var DraftEdit = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "mail", Command: "+draft-edit", Product: "mail",
	Description: "更新已有草稿并精确读回验证", Intent: "修改尚未发送的草稿；要求至少提供一个变化字段，并在写后按原 messageId 核对修改内容。",
	Risk: shortcut.RiskWrite, Safety: mailWriteSafety("unknown"),
	Contract: mailWriteContract("+draft-edit", "更新已有草稿并精确读回验证", "修改尚未发送的草稿；要求至少提供一个变化字段，并在写后按原 messageId 核对修改内容。", []contract.ParamDecl{{Name: "id", Property: "id"}, {Name: "from", Property: "from"}, {Name: "to", Property: "toRecipients"}, {Name: "cc", Property: "ccRecipients"}, {Name: "body", Property: "body"}}, `dws mail +draft-edit --from user@company.com --id <messageId> --subject "新主题" --format json`),
	Flags: []shortcut.Flag{
		{Name: "from", Type: shortcut.FlagString, Required: true, Desc: "发件邮箱"},
		{Name: "id", Type: shortcut.FlagString, Required: true, Desc: "草稿 messageId"},
		{Name: "to", Type: shortcut.FlagStringSlice, Desc: "新收件邮箱"},
		{Name: "cc", Type: shortcut.FlagStringSlice, Desc: "新抄送邮箱"},
		{Name: "subject", Type: shortcut.FlagString, Desc: "新主题"},
		{Name: "body", Type: shortcut.FlagString, Desc: "新正文"},
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		if !rt.Changed("to") && !rt.Changed("cc") && !rt.Changed("subject") && !rt.Changed("body") {
			return apperrors.NewValidation("至少指定 --to、--cc、--subject、--body 之一")
		}
		params := map[string]any{"from": rt.Str("from"), "id": rt.Str("id")}
		if rt.Changed("to") {
			params["toRecipients"] = mailRecipients(rt.StrSlice("to"))
		}
		if rt.Changed("cc") {
			params["ccRecipients"] = mailRecipients(rt.StrSlice("cc"))
		}
		if rt.Changed("subject") {
			params["subject"] = rt.Str("subject")
		}
		if rt.Changed("body") {
			params["body"] = rt.Str("body")
		}
		if rt.DryRun() {
			return rt.Output(map[string]any{"value": map[string]any{"dryRun": true, "executed": false, "operation": "mail/update_draft"}})
		}
		written, err := rt.CallMCPWriteDataStrict("mail", "update_draft", params)
		if err != nil {
			return err
		}
		receipt, err := mailDraftMessage(written, "mail/update_draft")
		if err != nil {
			return err
		}
		if mailFirstString(receipt, "id") != rt.Str("id") {
			return mailResponseError("mail/update_draft", "identity_mismatch", "更新回执的 messageId 与请求不一致")
		}
		verified, err := mailReadMessage(rt, rt.Str("from"), rt.Str("id"))
		if err != nil {
			return err
		}
		if rt.Changed("subject") && mailFirstString(verified, "subject") != rt.Str("subject") {
			return mailResponseError("mail/update_draft", "verification_mismatch", "草稿读回主题与请求不一致")
		}
		if rt.Changed("body") && mailFirstString(verified, "markdownBody", "body") != rt.Str("body") {
			return mailResponseError("mail/update_draft", "verification_mismatch", "草稿读回正文与请求不一致")
		}
		return rt.Output(map[string]any{"value": verified})
	},
}

var TemplateCreate = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "mail", Command: "+template-create", Product: "mail",
	Description: "创建个人邮件模板并按模板 ID 读回", Intent: "保存可复用邮件主题和正文；创建后按稳定模板 ID 读取并核对名称、主题和正文。",
	Risk: shortcut.RiskWrite, Safety: mailWriteSafety("non_idempotent"),
	Contract: mailWriteContract("+template-create", "创建个人邮件模板并按模板 ID 读回", "保存可复用邮件主题和正文；创建后按稳定模板 ID 读取并核对名称、主题和正文。", []contract.ParamDecl{{Name: "email", Property: "email"}, {Name: "from", Property: "from"}, {Name: "body", Property: "body"}, {Name: "draft", Property: "isDraft"}}, `dws mail +template-create --email user@company.com --name "模板" --subject "主题" --body "正文" --format json`),
	Flags: []shortcut.Flag{
		{Name: "email", Type: shortcut.FlagString, Required: true, Desc: "模板所属邮箱"},
		{Name: "from", Type: shortcut.FlagString, Desc: "模板发件邮箱"},
		{Name: "name", Type: shortcut.FlagString, Required: true, Desc: "模板名称"},
		{Name: "subject", Type: shortcut.FlagString, Required: true, Desc: "模板主题"},
		{Name: "body", Type: shortcut.FlagString, Required: true, Desc: "模板正文"},
		{Name: "draft", Type: shortcut.FlagBool, Desc: "创建可编辑的草稿模板"},
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{"email": rt.Str("email"), "name": rt.Str("name"), "subject": rt.Str("subject"), "body": rt.Str("body"), "isDraft": rt.Bool("draft")}
		if rt.Changed("from") {
			params["from"] = rt.Str("from")
		}
		if rt.DryRun() {
			return rt.Output(map[string]any{"value": map[string]any{"dryRun": true, "executed": false, "operation": "mail/create_user_message_template"}})
		}
		written, err := rt.CallMCPWriteDataStrict("mail", "create_user_message_template", params)
		if err != nil {
			return err
		}
		if err := mailRequireSuccess(written, "mail/create_user_message_template"); err != nil {
			return err
		}
		id := mailFirstString(written, "id")
		if id == "" {
			return mailResponseError("mail/create_user_message_template", "missing_stable_id", "创建模板响应缺少稳定 ID")
		}
		verified, err := mailReadTemplate(rt, rt.Str("email"), id)
		if err != nil {
			return err
		}
		if mailFirstString(verified, "name") != rt.Str("name") || mailTemplateSubject(verified) != rt.Str("subject") || mailTemplateBody(verified) != rt.Str("body") {
			return mailResponseError("mail/create_user_message_template", "verification_mismatch", "模板读回内容与请求不一致")
		}
		return rt.Output(map[string]any{"value": verified})
	},
}

var TemplateUpdate = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "mail", Command: "+template-update", Product: "mail",
	Description: "更新草稿模板并按原 ID 读回验证", Intent: "修改可编辑的草稿模板；写响应成功后按原模板 ID 读取并核对所有请求变更。",
	Risk: shortcut.RiskWrite, Safety: mailWriteSafety("unknown"),
	Contract: mailWriteContract("+template-update", "更新草稿模板并按原 ID 读回验证", "修改可编辑的草稿模板；写响应成功后按原模板 ID 读取并核对所有请求变更。", []contract.ParamDecl{{Name: "email", Property: "email"}, {Name: "id", Property: "id"}, {Name: "body", Property: "body"}}, `dws mail +template-update --email user@company.com --id <templateId> --subject "新主题" --format json`),
	Flags: []shortcut.Flag{
		{Name: "email", Type: shortcut.FlagString, Required: true, Desc: "模板所属邮箱"},
		{Name: "id", Type: shortcut.FlagString, Required: true, Desc: "模板 ID；仅草稿模板可更新"},
		{Name: "name", Type: shortcut.FlagString, Desc: "新模板名称"},
		{Name: "subject", Type: shortcut.FlagString, Desc: "新主题"},
		{Name: "body", Type: shortcut.FlagString, Desc: "新正文"},
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		if !rt.Changed("name") && !rt.Changed("subject") && !rt.Changed("body") {
			return apperrors.NewValidation("至少指定 --name、--subject、--body 之一")
		}
		params := map[string]any{"email": rt.Str("email"), "id": rt.Str("id")}
		for _, flag := range []string{"name", "subject", "body"} {
			if rt.Changed(flag) {
				params[flag] = rt.Str(flag)
			}
		}
		if rt.DryRun() {
			return rt.Output(map[string]any{"value": map[string]any{"dryRun": true, "executed": false, "operation": "mail/update_user_message_template"}})
		}
		written, err := rt.CallMCPWriteDataStrict("mail", "update_user_message_template", params)
		if err != nil {
			return err
		}
		if err := mailRequireSuccess(written, "mail/update_user_message_template"); err != nil {
			return err
		}
		verified, err := mailReadTemplate(rt, rt.Str("email"), rt.Str("id"))
		if err != nil {
			return err
		}
		if rt.Changed("name") && mailFirstString(verified, "name") != rt.Str("name") {
			return mailResponseError("mail/update_user_message_template", "verification_mismatch", "模板读回名称与请求不一致")
		}
		if rt.Changed("subject") && mailTemplateSubject(verified) != rt.Str("subject") {
			return mailResponseError("mail/update_user_message_template", "verification_mismatch", "模板读回主题与请求不一致")
		}
		if rt.Changed("body") && mailTemplateBody(verified) != rt.Str("body") {
			return mailResponseError("mail/update_user_message_template", "verification_mismatch", "模板读回正文与请求不一致")
		}
		return rt.Output(map[string]any{"value": verified})
	},
}

func init() {
	shortcut.Register(DraftCreate, DraftEdit, TemplateCreate, TemplateUpdate)
}
