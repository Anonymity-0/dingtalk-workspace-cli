// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	whiteboardServerID   = "whiteboard"
	whiteboardQueryTool  = "read_whiteboard_content"
	whiteboardUpdateTool = "update_whiteboard"
)

type whiteboardUpdateFile struct {
	Overwrite bool                  `json:"overwrite"`
	Source    *whiteboardOpenSource `json:"source"`
}

type whiteboardOpenSource struct {
	SchemaVersion  string          `json:"schemaVersion"`
	CatalogVersion string          `json:"catalogVersion"`
	Nodes          json.RawMessage `json:"nodes"`
}

func newWhiteboardCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "whiteboard",
		Short: "钉钉文档内嵌白板管理",
		Long: `读取或更新钉钉在线文档中已经存在的内嵌白板。

当前仅支持单页白板。每次操作都必须同时提供文档 ID 或 URL 和白板 part ID；
本命令不负责创建白板（请使用 dws doc whiteboard insert），也不支持通过已有节点 ID 做局部修改。`,
		RunE: groupRunE,
	}

	queryCmd := &cobra.Command{
		Use:     "query",
		Short:   "读取白板内容",
		Example: `  dws whiteboard query --node DOC_ID_OR_URL --part-id WHITEBOARD_PART_ID --format json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := rejectWhiteboardOutputFilters(cmd); err != nil {
				return err
			}
			if err := validateRequiredFlags(cmd, "node", "part-id"); err != nil {
				return err
			}
			return callWhiteboardTool(cmd, whiteboardQueryTool, map[string]any{
				"nodeId": mustGetFlag(cmd, "node"),
				"partId": mustGetFlag(cmd, "part-id"),
			})
		},
	}
	queryCmd.Flags().String("node", "", "承载白板的钉钉文档 ID 或 URL（必填）")
	queryCmd.Flags().String("part-id", "", "文档内白板 part ID（必填）")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "追加或整页重建白板内容",
		Long: `从 JSON 文件读取 OpenNodes V1 更新请求并更新已有白板。

更新模式由文件顶层的 overwrite 字段决定。overwrite=false 表示追加，
overwrite=true 表示整页重建。两种模式都会写入远端白板，必须同时传入 --yes。`,
		Example: `  dws whiteboard update --node DOC_ID_OR_URL --part-id WHITEBOARD_PART_ID --source ./whiteboard.json --format json
  dws whiteboard update --node DOC_ID_OR_URL --part-id WHITEBOARD_PART_ID --source ./overwrite.json --yes --format json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := rejectWhiteboardOutputFilters(cmd); err != nil {
				return err
			}
			if err := validateRequiredFlags(cmd, "node", "part-id", "source"); err != nil {
				return err
			}

			input, nodesJSON, err := loadWhiteboardUpdateFile(mustGetFlag(cmd, "source"))
			if err != nil {
				return err
			}
			if !deps.Caller.DryRun() && !confirmDangerousAction(cmd, "update whiteboard content", mustGetFlag(cmd, "part-id")) {
				return fmt.Errorf("whiteboard update cancelled")
			}

			mode := "append"
			if input.Overwrite {
				mode = "overwrite"
			}
			return callWhiteboardTool(cmd, whiteboardUpdateTool, map[string]any{
				"nodeId": mustGetFlag(cmd, "node"),
				"partId": mustGetFlag(cmd, "part-id"),
				"mode":   mode,
				"nodes":  nodesJSON,
			})
		},
	}
	updateCmd.Flags().String("node", "", "承载白板的钉钉文档 ID 或 URL（必填）")
	updateCmd.Flags().String("part-id", "", "文档内白板 part ID（必填）")
	updateCmd.Flags().String("source", "", "OpenNodes V1 更新请求 JSON 文件（必填）")
	updateCmd.Flags().Bool("yes", false, "确认写入远端白板")

	root.AddCommand(queryCmd, updateCmd)
	return root
}

func rejectWhiteboardOutputFilters(cmd *cobra.Command) error {
	for _, name := range []string{"jq", "fields"} {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			flag = cmd.InheritedFlags().Lookup(name)
		}
		if flag != nil && flag.Changed {
			return &CLIError{
				Code:       CodeInvalidParam,
				Message:    fmt.Sprintf("whiteboard 命令不支持 --%s", name),
				Suggestion: "直接读取命令返回的结构化 JSON",
			}
		}
	}
	return nil
}

func loadWhiteboardUpdateFile(path string) (*whiteboardUpdateFile, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		code := CodeInvalidPath
		if os.IsNotExist(err) {
			code = CodeFileNotFound
		}
		return nil, "", &CLIError{
			Code:       code,
			Message:    fmt.Sprintf("无法读取白板更新文件 %q", path),
			Suggestion: "确认 --source 指向可读的 UTF-8 JSON 文件",
			Cause:      err,
		}
	}

	var input whiteboardUpdateFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, "", invalidWhiteboardSourceJSON(err)
	}
	if err := ensureWhiteboardJSONEOF(decoder); err != nil {
		return nil, "", invalidWhiteboardSourceJSON(err)
	}
	if input.Source == nil {
		return nil, "", invalidWhiteboardSourceParam("source is required")
	}
	if input.Source.SchemaVersion != "1.0" {
		return nil, "", invalidWhiteboardSourceParam(`source.schemaVersion must be "1.0"`)
	}
	if input.Source.CatalogVersion != "dml-v1" {
		return nil, "", invalidWhiteboardSourceParam(`source.catalogVersion must be "dml-v1"`)
	}

	nodesJSON, nodeCount, err := validateWhiteboardNodes(input.Source.Nodes)
	if err != nil {
		return nil, "", err
	}
	if !input.Overwrite && nodeCount == 0 {
		return nil, "", invalidWhiteboardSourceParam("append requires at least one source.nodes item")
	}
	return &input, nodesJSON, nil
}

func ensureWhiteboardJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return fmt.Errorf("multiple JSON values are not allowed")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func validateWhiteboardNodes(raw json.RawMessage) (string, int, error) {
	if len(raw) == 0 || !strings.HasPrefix(strings.TrimSpace(string(raw)), "[") {
		return "", 0, invalidWhiteboardSourceParam("source.nodes must be an array")
	}

	var nodes []json.RawMessage
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return "", 0, invalidWhiteboardSourceJSON(err)
	}
	for i, node := range nodes {
		var object map[string]any
		if err := json.Unmarshal(node, &object); err != nil || object == nil {
			return "", 0, invalidWhiteboardSourceParam(fmt.Sprintf("source.nodes[%d] must be an object", i))
		}
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return "", 0, invalidWhiteboardSourceJSON(err)
	}
	return compact.String(), len(nodes), nil
}

func invalidWhiteboardSourceJSON(err error) error {
	return &CLIError{
		Code:       CodeInvalidJSON,
		Message:    "白板更新文件不是合法的 OpenNodes V1 JSON",
		Suggestion: "检查 JSON 语法、未知字段以及 source 对象结构",
		Cause:      err,
	}
}

func invalidWhiteboardSourceParam(message string) error {
	return &CLIError{
		Code:       CodeInvalidParam,
		Message:    message,
		Suggestion: "参考 whiteboard Skill 中的 OpenNodes V1 文件格式",
	}
}

func callWhiteboardTool(cmd *cobra.Command, toolName string, args map[string]any) error {
	if deps.Caller.DryRun() {
		return callMCPToolOnServer(whiteboardServerID, toolName, args)
	}

	text, err := callMCPToolReturnTextOnServer(cmd.Context(), whiteboardServerID, toolName, args)
	if err != nil {
		return err
	}
	if text == "" {
		return nil
	}

	var response map[string]any
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	if err := decoder.Decode(&response); err != nil {
		return invalidWhiteboardToolResult(toolName, err)
	}
	if err := ensureWhiteboardJSONEOF(decoder); err != nil {
		return invalidWhiteboardToolResult(toolName, err)
	}
	if response == nil {
		return invalidWhiteboardToolResult(toolName, fmt.Errorf("response must be a JSON object"))
	}

	if encoded, ok := response["resultJson"].(string); ok && strings.TrimSpace(encoded) != "" {
		var result any
		resultDecoder := json.NewDecoder(strings.NewReader(encoded))
		resultDecoder.UseNumber()
		if err := resultDecoder.Decode(&result); err != nil {
			return invalidWhiteboardToolResult(toolName, fmt.Errorf("invalid resultJson: %w", err))
		}
		if err := ensureWhiteboardJSONEOF(resultDecoder); err != nil {
			return invalidWhiteboardToolResult(toolName, fmt.Errorf("invalid resultJson: %w", err))
		}
		response["resultJson"] = result
	}
	return deps.Out.PrintJSON(response)
}

func invalidWhiteboardToolResult(toolName string, err error) error {
	return &CLIError{
		Code:       CodeMCPToolError,
		Message:    "白板服务返回了无法解析的 JSON",
		Suggestion: "使用 --debug 获取调用信息并联系白板服务维护者",
		Operation:  whiteboardServerID + "/" + toolName,
		Cause:      err,
	}
}
