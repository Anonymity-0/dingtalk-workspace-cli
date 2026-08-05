package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

// doc_whiteboard.go — `dws doc whiteboard insert` 白板卡片命令。
// 关联 SPEC: .qoder/docs/specs/doc-whiteboard-commands.md
//
// insert: 生成 blockUuid + whiteboardId(partId) → 插入固定 hetu draw card JSONML
//         → 回查验证 metadata.id 落库（线上 autoseed 只认已带 metadata.id 的
//         card，无 id 会被直接跳过，因此 id 必须客户端生成）。回查按
//         whiteboardRetryDelays 退避重试，最终拿不到时 fail-soft：输出
//         blockId + WARN，不报错。
// 删除白板卡片无专用命令：与普通块一致，走 `dws doc block delete`。

const whiteboardDrawPluginType = "application/x-alidocs-plugin-draw"

// whiteboardDefaultHeight 是现网 draw card 的默认卡片高度；card 恒带数字
// height 是 seed 依赖的合法形态。
const whiteboardDefaultHeight = 600

// whiteboardRetryDelays 是回查 metadata.id 的重试退避序列（首查后最多重试 len 次）。
// 包级变量仅供测试注入，不暴露为配置。
var whiteboardRetryDelays = []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}

// whiteboardSleep 供测试替换以避免真实等待。
var whiteboardSleep = time.Sleep

// buildWhiteboardCardJSONML 生成固定的空白板卡片 JSONML（单行 JSON 字符串）。
// blockUUID 与 whiteboardID(partId) 均由客户端显式提供：
//   - 白板定位依赖 uuid，不能依赖服务端补全；
//   - 服务端 autoseed 对缺 metadata.id 的 hetu card 直接跳过（不建 part
//     也不回填 id），因此 partId 必须随 card 一起写入。
func buildWhiteboardCardJSONML(blockUUID, whiteboardID string) string {
	node := []any{
		"card",
		map[string]any{
			"uuid":     blockUUID,
			"cardType": "hetu",
			"height":   whiteboardDefaultHeight,
			"metadata": map[string]any{"type": whiteboardDrawPluginType, "id": whiteboardID},
		},
		[]any{"span", map[string]any{"data-type": "text"},
			[]any{"span", map[string]any{"data-type": "leaf"}, ""}},
	}
	out, err := json.Marshal(node)
	if err != nil {
		// 固定模板 marshal 不应失败；防御性兜底。
		return ""
	}
	return string(out)
}

// extractWhiteboardID 从 card attrs 中提取 metadata.id（白板资源 ID）。
// 缺失 / 非 string / 空串均返回 ""。
func extractWhiteboardID(attrs map[string]any) string {
	meta, _ := attrs["metadata"].(map[string]any)
	if meta == nil {
		return ""
	}
	id, _ := meta["id"].(string)
	return id
}

// queryWhiteboardCardNode 调用 list_document_blocks 读取指定块的 JSONML 子树，
// 返回解析后的节点数组。响应兼容外层 {"result":{...}} 包裹
// （与 parseAttachmentUploadInfo 同策略）。
// 注意：服务端对 blockId 的过滤可能不生效（返回全文 blocks 列表），
// 因此这里按 blockId 精确匹配目标块，不盲取 blocks[0]。
func queryWhiteboardCardNode(ctx context.Context, nodeID, blockID string) ([]any, error) {
	text, err := callMCPToolReturnText(ctx, "list_document_blocks", map[string]any{
		"nodeId":  nodeID,
		"blockId": blockID,
		"format":  "jsonml",
	})
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return nil, fmt.Errorf("parse list_document_blocks response: %w", err)
	}
	if result, ok := data["result"].(map[string]any); ok {
		data = result
	}
	blocks, _ := data["blocks"].([]any)
	var raw string
	for _, b := range blocks {
		entry, _ := b.(map[string]any)
		if entry == nil || entry["blockId"] != blockID {
			continue
		}
		raw, _ = entry["jsonml"].(string)
		break
	}
	if raw == "" {
		return nil, fmt.Errorf("块 %s 不存在或查询无结果", blockID)
	}
	var node []any
	if err := json.Unmarshal([]byte(raw), &node); err != nil {
		return nil, fmt.Errorf("parse block jsonml: %w", err)
	}
	return node, nil
}

// queryWhiteboardCardAttrs 返回指定块 JSONML 节点的 attrs map。
func queryWhiteboardCardAttrs(ctx context.Context, nodeID, blockID string) (map[string]any, error) {
	node, err := queryWhiteboardCardNode(ctx, nodeID, blockID)
	if err != nil {
		return nil, err
	}
	if len(node) < 2 {
		return nil, fmt.Errorf("块 %s 的 jsonml 节点缺少 attrs", blockID)
	}
	attrs, _ := node[1].(map[string]any)
	if attrs == nil {
		return nil, fmt.Errorf("块 %s 的 jsonml attrs 不是对象", blockID)
	}
	return attrs, nil
}

func runWhiteboardInsert(cmd *cobra.Command, _ []string) error {
	nodeID, err := mustFlagOrFallback(cmd, "node", "url", "id", "node-id", "doc-id", "file-id")
	if err != nil {
		return err
	}

	blockUUID := uuid.New().String()
	// whiteboardId(partId) 同样客户端生成：现网 partId 为 UUID 形 8-4-4-4-12 hex，
	// uuid v4 形状兼容。
	whiteboardID := uuid.New().String()
	element := buildWhiteboardCardJSONML(blockUUID, whiteboardID)
	// 固定模板也过既有校验管线，防御 schema 演进导致的静默漂移。
	normalized, err := prepareJsonMLNode(cmd, element)
	if err != nil {
		return fmt.Errorf("内部错误: 白板卡片模板未通过 JSONML 校验: %w", err)
	}

	toolArgs := map[string]any{
		"nodeId": nodeID,
		"jsonml": normalized,
		"format": "jsonml",
	}
	if v, _ := cmd.Flags().GetString("ref-block"); v != "" {
		toolArgs["referenceBlockId"] = v
		where, _ := cmd.Flags().GetString("where")
		if where == "" {
			where = "after"
		}
		toolArgs["where"] = where
	}
	if v, _ := cmd.Flags().GetString("parent-block"); v != "" {
		toolArgs["referenceBlockId"] = v
	}
	if cmd.Flags().Changed("index") {
		idx, _ := cmd.Flags().GetInt("index")
		toolArgs["index"] = idx
	}

	if deps.Caller.DryRun() {
		return deps.Out.PrintJSON(map[string]any{
			"dry_run":  true,
			"executed": false,
			"tool":     "insert_document_block",
			"arguments": map[string]any{
				"nodeId":           toolArgs["nodeId"],
				"format":           "jsonml",
				"referenceBlockId": toolArgs["referenceBlockId"],
				"where":            toolArgs["where"],
				"index":            toolArgs["index"],
			},
			"note": "dry-run：将生成 blockUuid/whiteboardId 并插入 hetu draw card，随后回查 metadata.id",
		})
	}

	ctx := context.Background()
	deps.Out.PrintProgress("[1/2] 插入白板卡片...")
	if _, err := callMCPToolReturnText(ctx, "insert_document_block", toolArgs); err != nil {
		return err
	}

	deps.Out.PrintProgress("[2/2] 验证白板资源 ID 落库...")
	persistedID := ""
	for attempt := 0; attempt <= len(whiteboardRetryDelays); attempt++ {
		attrs, qErr := queryWhiteboardCardAttrs(ctx, nodeID, blockUUID)
		if qErr == nil {
			persistedID = extractWhiteboardID(attrs)
		}
		if persistedID != "" {
			break
		}
		if attempt < len(whiteboardRetryDelays) {
			whiteboardSleep(whiteboardRetryDelays[attempt])
		}
	}

	result := map[string]any{"blockId": blockUUID}
	if persistedID == "" {
		// fail-soft：插入已成功，不以错误退出。
		result["whiteboardId"] = nil
		deps.Out.PrintWarning(fmt.Sprintf(
			"白板已插入但未验证到 whiteboardId 落库，可稍后回查: dws doc block list --node %s --content-format jsonml --block-id %s",
			nodeID, blockUUID))
	} else {
		// 输出落库真值（正常应等于客户端生成值，服务端重写时以服务端为准）。
		result["whiteboardId"] = persistedID
	}
	return deps.Out.PrintJSON(map[string]any{"success": true, "result": result})
}

// newDocWhiteboardCommand 构建 `dws doc whiteboard` 命令组。
func newDocWhiteboardCommand() *cobra.Command {
	whiteboardCmd := &cobra.Command{
		Use:   "whiteboard",
		Short: "白板卡片管理",
		Long:  `管理钉钉文档中的白板卡片：插入空白板并获取白板资源 ID。删除白板卡片请使用 dws doc block delete。`,
		RunE:  groupRunE,
	}

	insertCmd := &cobra.Command{
		Use:   "insert",
		Short: "插入白板卡片",
		Long: `向文档插入一个空白板卡片（hetu draw card），并返回 blockId 与 whiteboardId。

流程:
  1. 生成卡片 uuid 与白板资源 ID（metadata.id），插入固定白板卡片 JSONML (insert_document_block)
  2. 按 uuid 回查块 (list_document_blocks)，验证 metadata.id 落库后返回

输出: {"success":true,"result":{"blockId":"<uuid>","whiteboardId":"<id>"}}
若未验证到 whiteboardId 落库，命令仍成功返回 blockId（whiteboardId 为 null），可稍后回查。`,
		Example: `  # 在文档末尾插入白板
  dws doc whiteboard insert --node DOC_ID

  # 在指定块之前插入
  dws doc whiteboard insert --node DOC_ID --ref-block BLOCK_ID --where before

  # 在容器内指定位置插入
  dws doc whiteboard insert --node DOC_ID --parent-block PARENT_ID --index 2`,
		RunE: runWhiteboardInsert,
	}
	insertCmd.Flags().String("node", "", "文档 ID 或 URL (必填)")
	insertCmd.Flags().String("ref-block", "", "参照块 UUID（同级插入，配合 --where）")
	insertCmd.Flags().String("where", "", "插入方向: before / after (默认 after，配合 --ref-block)")
	insertCmd.Flags().String("parent-block", "", "父容器 UUID（容器内插入，与 --index 配合）")
	insertCmd.Flags().Int("index", 0, "位置索引 (从 0 开始)")

	// --node 的隐藏别名（与 doc 其他子命令对齐）
	for _, c := range []*cobra.Command{insertCmd} {
		c.Flags().String("url", "", "--node 的别名")
		c.Flags().String("id", "", "--node 的别名")
		c.Flags().String("node-id", "", "--node 的别名")
		c.Flags().String("doc-id", "", "--node 的别名")
		c.Flags().String("file-id", "", "--node 的别名 (跨产品兼容 drive)")
		_ = c.Flags().MarkHidden("url")
		_ = c.Flags().MarkHidden("id")
		_ = c.Flags().MarkHidden("node-id")
		_ = c.Flags().MarkHidden("doc-id")
		_ = c.Flags().MarkHidden("file-id")
	}

	DeclareLeafMetadata(insertCmd, LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "medium",
			Confirmation: "not_required", Idempotency: "non_idempotent",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "doc",
				Name:           "whiteboard_insert",
				CanonicalPath:  "doc.whiteboard_insert",
				CLIPath:        "doc whiteboard insert",
				PrimaryCLIPath: "doc whiteboard insert",
			},
			Description: "向文档插入空白板卡片并返回 blockId/whiteboardId",
			DryRun:      &contract.DryRunSpec{PreviewKind: "plan", RemoteReads: false},
			Interface: &contract.InterfaceSpec{
				Mode:         "composite",
				Availability: "available",
				Reason:       "Client generates card JSONML, calls insert_document_block, then verifies metadata.id via list_document_blocks.",
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "在文档中插入空白板卡片",
				UseWhen:      []string{"需要在钉钉文档正文插入空白板（hetu draw）卡片时"},
				AvoidWhen:    []string{"删除白板卡片请用 doc block delete；普通附件插入用 doc media insert"},
				Examples:     []string{"dws doc whiteboard insert --node <DOC_ID>"},
			},
			Parameters: []contract.ParamDecl{
				{Name: "node", Property: "nodeId", Required: boolPtr(true)},
				{Name: "ref-block", Property: "referenceBlockId"},
				{Name: "where", Property: "where"},
				{Name: "parent-block", Property: "parentBlockId"},
				{Name: "index", Property: "index", InterfaceType: "integer"},
			},
		},
	})
	whiteboardCmd.AddCommand(insertCmd)
	return whiteboardCmd
}
