// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

func parseOAAttachmentFileInfos(raw string) ([]map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var items []map[string]json.RawMessage
	if err := decoder.Decode(&items); err != nil {
		return nil, fmt.Errorf("--file-infos JSON 解析失败: %w", err)
	}
	if err := rejectTrailingOAAttachmentJSON(decoder); err != nil {
		return nil, err
	}
	if len(items) < 1 || len(items) > 10 {
		return nil, fmt.Errorf("--file-infos 必须包含 1 至 10 个文件")
	}

	infos := make([]map[string]any, 0, len(items))
	for index, item := range items {
		for name := range item {
			if name != "spaceId" && name != "fileId" {
				return nil, fmt.Errorf("--file-infos 第 %d 项包含未知字段 %q", index+1, name)
			}
		}

		spaceRaw, ok := item["spaceId"]
		if !ok {
			return nil, fmt.Errorf("--file-infos 第 %d 项缺少 spaceId", index+1)
		}
		spaceDecoder := json.NewDecoder(strings.NewReader(string(spaceRaw)))
		spaceDecoder.UseNumber()
		var spaceValue any
		if err := spaceDecoder.Decode(&spaceValue); err != nil {
			return nil, fmt.Errorf("--file-infos 第 %d 项 spaceId 不是有效数字: %w", index+1, err)
		}
		spaceID, ok := spaceValue.(json.Number)
		if !ok {
			return nil, fmt.Errorf("--file-infos 第 %d 项 spaceId 必须是数字", index+1)
		}

		fileRaw, ok := item["fileId"]
		if !ok {
			return nil, fmt.Errorf("--file-infos 第 %d 项缺少 fileId", index+1)
		}
		var fileID string
		if err := json.Unmarshal(fileRaw, &fileID); err != nil {
			return nil, fmt.Errorf("--file-infos 第 %d 项 fileId 必须是字符串: %w", index+1, err)
		}
		fileID = strings.TrimSpace(fileID)
		if fileID == "" {
			return nil, fmt.Errorf("--file-infos 第 %d 项 fileId 不能为空", index+1)
		}

		infos = append(infos, map[string]any{"spaceId": spaceID, "fileId": fileID})
	}
	return infos, nil
}

func rejectTrailingOAAttachmentJSON(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("--file-infos JSON 包含多余内容")
		}
		return fmt.Errorf("--file-infos JSON 解析失败: %w", err)
	}
	return nil
}

func validateOAAttachmentFileInfos(cmd *cobra.Command, _ []string) error {
	raw, _ := cmd.Flags().GetString("file-infos")
	_, err := parseOAAttachmentFileInfos(raw)
	return err
}

func validateOAPreviewFileIDs(cmd *cobra.Command, _ []string) error {
	fileIDs, _ := cmd.Flags().GetStringSlice("file-ids")
	if len(fileIDs) > 20 {
		return fmt.Errorf("--file-ids 最多包含 20 个附件 ID")
	}
	for index, fileID := range fileIDs {
		if strings.TrimSpace(fileID) == "" {
			return fmt.Errorf("--file-ids 第 %d 项不能为空", index+1)
		}
	}
	return nil
}

func newOAAttachmentCommand() *cobra.Command {
	attachmentCmd := &cobra.Command{
		Use:   "attachment",
		Short: "审批附件授权与下载链接",
		RunE:  groupRunE,
	}

	downloadURLCmd := NewLeafCommand(LeafSpec{
		Use:     "download-url",
		Short:   "获取审批附件下载链接",
		Example: "  dws oa approval attachment download-url --instance-id <processInstanceId> --file-id <fileId>",
		Server:  "oa",
		Tool:    "get_attachment_download_url",
		Call: func(_ *cobra.Command, tool string, args map[string]any) error {
			jsonOutput := strings.EqualFold(strings.TrimSpace(deps.Caller.Format()), "json")
			return callMCPToolInternalOpts("oa", tool, args, jsonOutput)
		},
		Flags: []LeafFlag{
			{Name: "instance-id", Usage: "审批实例 ID (必填)", Bind: "processInstanceId", Trim: true, Required: true, MarkRequired: true},
			{Name: "file-id", Usage: "审批附件文件 ID (必填)", Bind: "fileId", Trim: true, Required: true, MarkRequired: true},
			{Name: "with-comment-attachment", Usage: "是否包含评论中的附件", Kind: LeafBool, Bind: "withCommentAttachment"},
		},
		Safety: contract.SafetySpec{
			Effect: "read", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "oa",
				Name:           "get_attachment_download_url",
				CanonicalPath:  "oa.get_attachment_download_url",
				CLIPath:        "oa approval attachment download-url",
				PrimaryCLIPath: "oa approval attachment download-url",
			},
			Description: "获取审批附件下载授权并生成临时下载链接",
			Interface: &contract.InterfaceSpec{
				Mode:         "mcp",
				Availability: "available",
				Ref:          &contract.InterfaceRefSpec{ProductID: "oa", RPCName: "get_attachment_download_url"},
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "获取审批实例中指定附件的临时下载链接",
				UseWhen:      []string{"已从审批详情获得 processInstanceId 和 fileId，需要获取附件下载链接时"},
				AvoidWhen: []string{
					"只需查看审批表单和附件元数据时使用 dws oa approval detail",
					"需要将附件真正保存到本地时不要误认为本命令会下载文件；它只返回链接",
				},
				Examples: []string{"dws oa approval attachment download-url --instance-id <processInstanceId> --file-id <fileId>"},
			},
			Parameters: []contract.ParamDecl{
				{Name: "instance-id", Property: "processInstanceId", InterfaceType: "string"},
				{Name: "file-id", Property: "fileId", InterfaceType: "string"},
				{Name: "with-comment-attachment", Property: "withCommentAttachment", InterfaceType: "boolean"},
			},
		},
	})

	authorizeDownloadCmd := NewLeafCommand(LeafSpec{
		Use:     "authorize-download",
		Short:   "授权当前用户下载审批钉盘文件",
		Long:    "批量授权当前用户下载指定的审批钉盘文件。",
		Example: `  dws oa approval attachment authorize-download --file-infos '[{"spaceId":27827223951,"fileId":"232271651278"}]'`,
		Server:  "oa",
		Tool:    "auth_download_file",
		Flags: []LeafFlag{
			{
				Name: "file-infos", Usage: "审批钉盘文件信息 JSON 数组 (必填)",
				Bind: "fileInfos", Trim: true, Required: true, MarkRequired: true,
				Transform: func(raw string) (any, error) {
					return parseOAAttachmentFileInfos(raw)
				},
			},
		},
		Constraints: []LeafConstraint{{
			Kind: corecmd.Custom, Flags: []string{"file-infos"},
			Description: "文件信息列表必须包含 1 至 10 项",
		}},
		Validate: validateOAAttachmentFileInfos,
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "oa",
				Name:           "auth_download_file",
				CanonicalPath:  "oa.auth_download_file",
				CLIPath:        "oa approval attachment authorize-download",
				PrimaryCLIPath: "oa approval attachment authorize-download",
			},
			Description: "批量授权当前用户下载指定的审批钉盘文件",
			Interface: &contract.InterfaceSpec{
				Mode:         "mcp",
				Availability: "available",
				Ref:          &contract.InterfaceRefSpec{ProductID: "oa", RPCName: "auth_download_file"},
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "使用审批钉盘 spaceId/fileId 批量授权当前用户下载文件",
				UseWhen:      []string{"已有一个或多个审批钉盘文件的 spaceId 和 fileId，需要为当前用户取得下载权限时"},
				AvoidWhen: []string{
					"需要生成单个审批附件下载链接时使用 attachment download-url",
					"需要授权在审批单内预览附件时使用 attachment authorize-preview",
				},
				Examples: []string{`dws oa approval attachment authorize-download --file-infos '[{"spaceId":27827223951,"fileId":"232271651278"}]'`},
			},
			Parameters: []contract.ParamDecl{
				{Name: "file-infos", Property: "fileInfos", InterfaceType: "array"},
			},
		},
	})

	authorizePreviewCmd := NewLeafCommand(LeafSpec{
		Use:     "authorize-preview",
		Short:   "授权当前用户预览审批附件",
		Long:    "批量授权当前用户预览审批单中的附件。",
		Example: "  dws oa approval attachment authorize-preview --instance-id <processInstanceId> --file-ids <fileId1>,<fileId2>",
		Server:  "oa",
		Tool:    "auth_preview_attachment",
		Flags: []LeafFlag{
			{Name: "instance-id", Usage: "审批实例 ID (必填)", Bind: "processInstanceId", Trim: true, Required: true, MarkRequired: true},
			{Name: "file-ids", Usage: "附件 ID 列表，多个用逗号分隔 (必填)", Kind: LeafStringSlice, Bind: "fileIdList", Required: true, MarkRequired: true},
			{Name: "with-comment-attachment", Usage: "是否包含评论中的附件", Kind: LeafBool, Bind: "withCommentAttachment"},
		},
		Constraints: []LeafConstraint{{
			Kind: corecmd.Custom, Flags: []string{"file-ids"},
			Description: "附件 ID 列表最多包含 20 项且每项不能为空",
		}},
		Validate: validateOAPreviewFileIDs,
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "oa",
				Name:           "auth_preview_attachment",
				CanonicalPath:  "oa.auth_preview_attachment",
				CLIPath:        "oa approval attachment authorize-preview",
				PrimaryCLIPath: "oa approval attachment authorize-preview",
			},
			Description: "批量授权当前用户预览审批单中的附件",
			Interface: &contract.InterfaceSpec{
				Mode:         "mcp",
				Availability: "available",
				Ref:          &contract.InterfaceRefSpec{ProductID: "oa", RPCName: "auth_preview_attachment"},
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "按审批实例和附件 ID 列表授权当前用户预览附件",
				UseWhen:      []string{"已有 processInstanceId 和附件 fileId 列表，需要在审批场景中批量取得预览权限时"},
				AvoidWhen: []string{
					"需要下载权限而不是预览权限时使用 attachment authorize-download",
					"需要直接获得单个附件临时下载链接时使用 attachment download-url",
				},
				Examples: []string{"dws oa approval attachment authorize-preview --instance-id <processInstanceId> --file-ids <fileId1>,<fileId2>"},
			},
			Parameters: []contract.ParamDecl{
				{Name: "instance-id", Property: "processInstanceId", InterfaceType: "string"},
				{Name: "file-ids", Property: "fileIdList", InterfaceType: "array"},
				{Name: "with-comment-attachment", Property: "withCommentAttachment", InterfaceType: "boolean"},
			},
		},
	})

	attachmentCmd.AddCommand(downloadURLCmd, authorizeDownloadCmd, authorizePreviewCmd)
	return attachmentCmd
}
