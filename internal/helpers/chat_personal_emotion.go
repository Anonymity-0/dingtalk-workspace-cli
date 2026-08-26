package helpers

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/targetresolver"
	"github.com/spf13/cobra"
)

const personalEmotionUnpinnedReason = "Reviewed unpinned remote adapter: this executable CLI wrapper calls a remote helper that is absent from the pinned MCP metadata snapshot; no single pinned semantically equivalent interface_ref can represent the command."

func newChatEmotionCommand() *cobra.Command {
	cmd := newGroupCommand(&cobra.Command{
		Use:   "emotion",
		Short: "个人收藏表情",
		Long:  "查询、发送和新增当前用户的个人收藏表情。",
		RunE:  groupRunE,
	})
	cmd.AddCommand(
		newChatEmotionListCommand(),
		newChatEmotionSendCommand(),
		newChatEmotionFavoriteCommand(),
	)
	return cmd
}

func newChatEmotionListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出个人收藏表情",
		RunE: func(cmd *cobra.Command, args []string) error {
			return callMCPToolOnServer("im", "list_personal_emotions", map[string]any{})
		},
	}
	DeclareLeafMetadata(cmd, LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "idempotent",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID: "chat", Name: "list_personal_emotions",
				CanonicalPath: "chat.list_personal_emotions", CLIPath: "chat emotion list", PrimaryCLIPath: "chat emotion list",
			},
			Description: "列出当前用户的个人收藏表情",
			Interface: &contract.InterfaceSpec{
				Mode: "composite", Availability: "available", Reason: personalEmotionUnpinnedReason,
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "列出当前用户的个人收藏表情",
				UseWhen:      []string{"需要查看当前用户已收藏的表情、获取 emotionId 或 mediaId 时"},
				AvoidWhen:    []string{"查询消息 reaction 使用 chat message list-emotion-replies"},
				Examples:     []string{"dws chat emotion list --format json"},
			},
		},
	})
	return cmd
}

func newChatEmotionSendCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send",
		Short: "发送个人收藏表情",
		Long: `发送当前用户的个人收藏表情。

⚠️ 重要：该接口会真实发送表情到目标会话，不可用于测试或试探性调用。调用前必须确认表情媒体 ID 和接收对象无误。`,
		Example: `  dws chat emotion send --media-id <mediaId> --group <openConversationId>
  dws chat emotion send --media-id <mediaId> --emotion-id <emotionId> --user <userId>
  dws chat emotion send --media-id <mediaId> --open-dingtalk-id <openDingTalkId> --uuid <idempotencyKey>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mediaID, _ := cmd.Flags().GetString("media-id")
			if strings.TrimSpace(mediaID) == "" {
				return fmt.Errorf("--media-id is required")
			}
			target, err := personalEmotionSendTarget(cmd)
			if err != nil {
				return err
			}
			payload := map[string]any{"mediaId": strings.TrimSpace(mediaID)}
			emotionID, _ := cmd.Flags().GetString("emotion-id")
			if strings.TrimSpace(emotionID) != "" {
				payload["emotionId"] = strings.TrimSpace(emotionID)
			}
			for key, value := range target {
				payload[key] = value
			}
			if uuid := strings.TrimSpace(flagOrFallback(cmd, "uuid", "idempotency-key")); uuid != "" {
				payload["uuid"] = uuid
			}
			return callMCPToolOnServer("im", "send_personal_emotion", payload)
		},
	}
	cmd.Flags().String("media-id", "", "表情媒体 ID (必填)")
	cmd.Flags().String("emotion-id", "", "表情 ID")
	cmd.Flags().String("conversation-id", "", "群聊 openConversationId")
	cmd.Flags().String("group", "", "群聊 openConversationId（--conversation-id 别名）")
	cmd.Flags().String("user", "", "单聊接收人 userId；CLI 会解析为 openDingTalkId")
	cmd.Flags().String("open-dingtalk-id", "", "单聊接收人 openDingTalkId")
	cmd.Flags().String("uuid", "", "幂等键")
	cmd.Flags().String("idempotency-key", "", "幂等键（--uuid 别名）")
	cmd.MarkFlagsMutuallyExclusive("conversation-id", "group")
	cli.AnnotateRuntimeConstraints(cmd, cli.RuntimeSchemaConstraints{
		MutuallyExclusive: [][]string{{"conversation-id", "group", "user", "open-dingtalk-id"}},
		RequireOneOf:      [][]string{{"conversation-id", "group", "user", "open-dingtalk-id"}},
	})
	DeclareLeafMetadata(cmd, LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "medium", Confirmation: "not_required", Idempotency: "unknown",
		},
		Contract: personalEmotionSendContract(),
	})
	return cmd
}

func newChatEmotionFavoriteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "favorite",
		Short: "新增个人收藏表情",
		Example: `  dws chat emotion favorite --media-id <mediaId> --name "赞"
  dws chat emotion favorite --media-id <mediaId> --source-conversation-id <cid> --source-message-id <mid>
  dws chat emotion favorite --file-path ./sticker.png --name "本地表情"`,
		RunE: runChatEmotionFavorite,
	}
	cmd.Flags().String("media-id", "", "待收藏 mediaId；与 --file-path 二选一必填")
	cmd.Flags().String("file-path", "", "本地图片路径 (jpg/jpeg/png/gif/webp/bmp，≤10MB)；与 --media-id 二选一必填")
	cmd.Flags().String("name", "", "表情名称")
	cmd.Flags().String("source-conversation-id", "", "来源会话 ID；需与 --source-message-id 成对指定")
	cmd.Flags().String("source-message-id", "", "来源消息 ID；需与 --source-conversation-id 成对指定")
	cmd.MarkFlagsMutuallyExclusive("media-id", "file-path")
	cli.AnnotateRuntimeConstraints(cmd, cli.RuntimeSchemaConstraints{
		MutuallyExclusive: [][]string{{"media-id", "file-path"}},
		RequireOneOf:      [][]string{{"media-id", "file-path"}},
	})
	DeclareLeafMetadata(cmd, LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "medium", Confirmation: "not_required", Idempotency: "unknown",
		},
		Contract: personalEmotionFavoriteContract(),
	})
	return cmd
}

func runChatEmotionFavorite(cmd *cobra.Command, _ []string) error {
	mediaID, _ := cmd.Flags().GetString("media-id")
	mediaID = strings.TrimSpace(mediaID)
	filePath, _ := cmd.Flags().GetString("file-path")
	filePath = strings.TrimSpace(filePath)
	if mediaID == "" && filePath == "" {
		return fmt.Errorf("one of --media-id or --file-path is required")
	}
	sourceConversationID, _ := cmd.Flags().GetString("source-conversation-id")
	sourceMessageID, _ := cmd.Flags().GetString("source-message-id")
	if err := validatePersonalEmotionSourcePair(sourceConversationID, sourceMessageID); err != nil {
		return err
	}

	if filePath != "" {
		image, err := loadPersonalEmotionImageFile(filePath)
		if err != nil {
			return err
		}
		if deps.Caller.DryRun() {
			deps.Out.PrintKeyValue("操作", "上传本地图片并新增个人收藏表情")
			deps.Out.PrintKeyValue("文件", image.path)
			deps.Out.PrintKeyValue("大小", fmt.Sprintf("%d bytes", image.size))
			deps.Out.PrintKeyValue("图片类型", image.imageType)
			return nil
		}
		uploadedMediaID, err := uploadPersonalEmotionImage(cmd.Context(), image)
		if err != nil {
			return err
		}
		payload := buildPersonalEmotionFavoritePayload(uploadedMediaID, cmd)
		if err := callMCPToolOnServer("im", "favorite_personal_emotion", payload); err != nil {
			return fmt.Errorf("本地图片已上传 (mediaId=%s)，但收藏失败: %w；可用 --media-id %s 重试收藏，无需重新上传", uploadedMediaID, err, uploadedMediaID)
		}
		return nil
	}

	payload := buildPersonalEmotionFavoritePayload(mediaID, cmd)
	return callMCPToolOnServer("im", "favorite_personal_emotion", payload)
}

func buildPersonalEmotionFavoritePayload(mediaID string, cmd *cobra.Command) map[string]any {
	payload := map[string]any{"mediaId": mediaID}
	name, _ := cmd.Flags().GetString("name")
	if strings.TrimSpace(name) != "" {
		payload["name"] = strings.TrimSpace(name)
	}
	sourceConversationID, _ := cmd.Flags().GetString("source-conversation-id")
	sourceMessageID, _ := cmd.Flags().GetString("source-message-id")
	if strings.TrimSpace(sourceConversationID) != "" {
		payload["sourceConversationId"] = strings.TrimSpace(sourceConversationID)
		payload["sourceMessageId"] = strings.TrimSpace(sourceMessageID)
	}
	return payload
}

func personalEmotionSendTarget(cmd *cobra.Command) (map[string]any, error) {
	groupID := strings.TrimSpace(flagOrFallback(cmd, "conversation-id", "group"))
	userID, _ := cmd.Flags().GetString("user")
	openDingTalkID, _ := cmd.Flags().GetString("open-dingtalk-id")
	userID = strings.TrimSpace(userID)
	openDingTalkID = strings.TrimSpace(openDingTalkID)
	specified := 0
	for _, value := range []string{groupID, userID, openDingTalkID} {
		if value != "" {
			specified++
		}
	}
	if specified != 1 {
		return nil, fmt.Errorf("--conversation-id, --user or --open-dingtalk-id is required; specify exactly one")
	}
	if groupID != "" {
		return map[string]any{"openConversationId": groupID}, nil
	}
	if openDingTalkID != "" {
		if err := targetresolver.ValidateExplicitOpenDingTalkID("--open-dingtalk-id", openDingTalkID); err != nil {
			return nil, err
		}
		return map[string]any{"receiverOpenDingTalkId": openDingTalkID}, nil
	}
	if isOpenDingTalkID(userID) {
		return map[string]any{"receiverOpenDingTalkId": userID}, nil
	}
	resolved, err := resolveOpenDingTalkID(cmd.Context(), userID)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve --user %q to openDingTalkId: %w; pass --open-dingtalk-id instead", userID, err)
	}
	return map[string]any{"receiverOpenDingTalkId": resolved}, nil
}

func validatePersonalEmotionSourcePair(sourceConversationID, sourceMessageID string) error {
	hasConversation := strings.TrimSpace(sourceConversationID) != ""
	hasMessage := strings.TrimSpace(sourceMessageID) != ""
	if hasConversation != hasMessage {
		return fmt.Errorf("--source-conversation-id and --source-message-id must be specified together")
	}
	return nil
}

const personalEmotionImageMaxBytes = 10 * 1024 * 1024

// personalEmotionImageTypes maps lowercased file extensions to the
// im/upload_media imageType enum. jpg and jpeg stay distinct values because
// the remote tool echoes them separately.
var personalEmotionImageTypes = map[string]string{
	".jpg":  "jpg",
	".jpeg": "jpeg",
	".png":  "png",
	".gif":  "gif",
	".webp": "webp",
	".bmp":  "bmp",
}

func personalEmotionImageType(ext string) (string, bool) {
	imageType, ok := personalEmotionImageTypes[strings.ToLower(strings.TrimSpace(ext))]
	return imageType, ok
}

type personalEmotionImage struct {
	path      string
	size      int64
	imageType string
	content   string
}

func validatePersonalEmotionImageFile(filePath string) (int64, string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, "", fmt.Errorf("--file-path cannot read local image %s: %w", filePath, err)
	}
	if info.IsDir() {
		return 0, "", fmt.Errorf("--file-path points to a directory, expected an image file: %s", filePath)
	}
	if info.Size() > personalEmotionImageMaxBytes {
		return 0, "", fmt.Errorf("--file-path image size %d bytes exceeds the 10MB limit: %s", info.Size(), filePath)
	}
	imageType, ok := personalEmotionImageType(filepath.Ext(filePath))
	if !ok {
		return 0, "", fmt.Errorf("--file-path only supports jpg/jpeg/png/gif/webp/bmp images, got: %s", filePath)
	}
	return info.Size(), imageType, nil
}

func loadPersonalEmotionImageFile(filePath string) (*personalEmotionImage, error) {
	size, imageType, err := validatePersonalEmotionImageFile(filePath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("--file-path cannot read local image %s: %w", filePath, err)
	}
	if int64(len(data)) > personalEmotionImageMaxBytes {
		return nil, fmt.Errorf("--file-path image size %d bytes exceeds the 10MB limit: %s", len(data), filePath)
	}
	return &personalEmotionImage{
		path:      filePath,
		size:      size,
		imageType: imageType,
		content:   base64.StdEncoding.EncodeToString(data),
	}, nil
}

func uploadPersonalEmotionImage(ctx context.Context, image *personalEmotionImage) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	text, err := callMCPToolReturnTextOnServer(ctx, "im", "upload_media", map[string]any{
		"content":   image.content,
		"imageType": image.imageType,
		"bizType":   "chat_image",
	})
	if err != nil {
		return "", fmt.Errorf("上传本地图片到 im/upload_media 失败: %w", err)
	}
	return parsePersonalEmotionUploadMediaID(text)
}

func parsePersonalEmotionUploadMediaID(text string) (string, error) {
	var payload struct {
		Success   bool   `json:"success"`
		MediaIDV1 string `json:"mediaIdV1"`
		MediaIDV2 string `json:"mediaIdV2"`
		ErrorCode string `json:"errorCode"`
		ErrorMsg  string `json:"errorMsg"`
		LogID     string `json:"logId"`
		Message   string `json:"message"`
	}
	if err := unmarshalJSONUseNumber(text, &payload); err != nil {
		return "", fmt.Errorf("im/upload_media 返回值无法解析为 JSON: %w", err)
	}
	if !payload.Success {
		detail := strings.TrimSpace(payload.ErrorMsg)
		if detail == "" {
			detail = payload.Message
		}
		code := strings.TrimSpace(payload.ErrorCode)
		if code != "" {
			detail = strings.TrimSpace(detail + " (errorCode=" + code + ")")
		}
		if detail == "" {
			detail = "服务端未返回错误详情"
		}
		if logID := strings.TrimSpace(payload.LogID); logID != "" {
			detail = strings.TrimSpace(detail + " (logId=" + logID + ")")
		}
		return "", fmt.Errorf("im/upload_media 上传失败: %s", detail)
	}
	mediaID := strings.TrimSpace(payload.MediaIDV1)
	if mediaID == "" {
		return "", fmt.Errorf("im/upload_media 返回 success=true 但缺少 mediaIdV1 (mediaIdV2=%s)；个人收藏表情链路要求 V1 mediaId，拒绝降级使用 V2", strings.TrimSpace(payload.MediaIDV2))
	}
	return mediaID, nil
}

func personalEmotionSendContract() LeafContract {
	return LeafContract{
		Identity: contract.ToolIdentitySpec{
			ProductID: "chat", Name: "send_personal_emotion",
			CanonicalPath: "chat.send_personal_emotion", CLIPath: "chat emotion send", PrimaryCLIPath: "chat emotion send",
		},
		Description: "以当前用户身份向群聊或单聊发送个人收藏表情",
		Interface: &contract.InterfaceSpec{
			Mode: "composite", Availability: "available", Reason: personalEmotionUnpinnedReason,
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "以当前用户身份发送个人收藏表情",
			UseWhen:      []string{"用户明确要求发送个人收藏表情，且已提供 mediaId 或 emotionId 时"},
			AvoidWhen:    []string{"发送普通文本、Markdown 或文件时使用 chat message send"},
			Examples:     []string{"dws chat emotion send --media-id <mediaId> --group <openConversationId> --uuid <idempotencyKey>"},
		},
		Parameters: []contract.ParamDecl{
			{Name: "media-id", Property: "mediaId", Required: boolPtr(true)},
			{Name: "emotion-id", Property: "emotionId", Required: boolPtr(false)},
			{Name: "conversation-id", Property: "openConversationId", Required: boolPtr(false)},
			{Name: "group", Property: "openConversationId", Required: boolPtr(false)},
			{Name: "user", Property: "receiverOpenDingTalkId", Required: boolPtr(false)},
			{Name: "open-dingtalk-id", Property: "receiverOpenDingTalkId", Required: boolPtr(false)},
			{Name: "uuid", Property: "uuid", Required: boolPtr(false)},
			{Name: "idempotency-key", Property: "uuid", Required: boolPtr(false)},
		},
	}
}

func personalEmotionFavoriteContract() LeafContract {
	return LeafContract{
		Identity: contract.ToolIdentitySpec{
			ProductID: "chat", Name: "favorite_personal_emotion",
			CanonicalPath: "chat.favorite_personal_emotion", CLIPath: "chat emotion favorite", PrimaryCLIPath: "chat emotion favorite",
		},
		Description: "将 mediaId 或本地图片新增到当前用户的个人收藏表情（本地图片经 im/upload_media 上传后收藏）",
		Interface: &contract.InterfaceSpec{
			Mode: "composite", Availability: "available", Reason: personalEmotionUnpinnedReason,
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "新增当前用户的个人收藏表情，支持已有 mediaId 或本地图片文件",
			UseWhen:      []string{"用户要把一个 mediaId 或本地图片文件 (jpg/jpeg/png/gif/webp/bmp，≤10MB) 收藏为个人表情时"},
			AvoidWhen:    []string{"收藏消息使用 chat message add-favorite"},
			Examples: []string{
				`dws chat emotion favorite --media-id <mediaId> --name "赞"`,
				"dws chat emotion favorite --file-path ./sticker.png --name \"本地表情\"",
			},
		},
		Parameters: []contract.ParamDecl{
			{Name: "media-id", Property: "mediaId", Required: boolPtr(false)},
			{Name: "file-path", Required: boolPtr(false)},
			{Name: "name", Property: "name", Required: boolPtr(false)},
			{Name: "source-conversation-id", Property: "sourceConversationId", Required: boolPtr(false), RequiredWhen: "source-message-id is provided"},
			{Name: "source-message-id", Property: "sourceMessageId", Required: boolPtr(false), RequiredWhen: "source-conversation-id is provided"},
		},
	}
}
