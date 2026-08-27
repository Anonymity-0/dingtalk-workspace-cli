package helpers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type personalEmotionCall struct {
	server string
	tool   string
	args   map[string]any
}

type personalEmotionCaller struct {
	calls []personalEmotionCall
}

func (c *personalEmotionCaller) CallTool(_ context.Context, server, tool string, args map[string]any) (*edition.ToolResult, error) {
	copied := make(map[string]any, len(args))
	for key, value := range args {
		copied[key] = value
	}
	c.calls = append(c.calls, personalEmotionCall{server: server, tool: tool, args: copied})
	if server == "contact" && tool == "get_user_info_by_user_ids" {
		return textToolResult(`{"result":[{"userId":"u1","openDingTalkId":"` + helperCurrentDOpenID2 + `"}]}`), nil
	}
	return textToolResult(`{"ok":true}`), nil
}

func (*personalEmotionCaller) Format() string { return "json" }
func (*personalEmotionCaller) DryRun() bool   { return false }
func (*personalEmotionCaller) Fields() string { return "" }
func (*personalEmotionCaller) JQ() string     { return "" }

func executePersonalEmotionCommand(t *testing.T, caller *personalEmotionCaller, args ...string) error {
	t.Helper()
	installHelpersCoreDeps(t, caller)
	deps.Out.w = io.Discard
	deps.Out.errW = io.Discard
	root := newChatCommand()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	return root.ExecuteContext(context.Background())
}

func requirePersonalEmotionCall(t *testing.T, caller *personalEmotionCaller, tool string, want map[string]any) {
	t.Helper()
	if len(caller.calls) != 1 {
		t.Fatalf("calls = %d, want 1: %+v", len(caller.calls), caller.calls)
	}
	call := caller.calls[0]
	if call.server != "im" || call.tool != tool {
		t.Fatalf("tool call = %s/%s, want im/%s", call.server, call.tool, tool)
	}
	if !reflect.DeepEqual(call.args, want) {
		t.Fatalf("args = %#v, want %#v", call.args, want)
	}
}

func TestChatEmotionListCallsIMToolWithoutBusinessArgs(t *testing.T) {
	// TC-001: list 无业务参数，当前用户身份由 MCP server 注入。
	caller := &personalEmotionCaller{}
	if err := executePersonalEmotionCommand(t, caller, "emotion", "list"); err != nil {
		t.Fatalf("chat emotion list returned error: %v", err)
	}
	requirePersonalEmotionCall(t, caller, "list_personal_emotions", map[string]any{})
}

func TestChatEmotionSendMapsGroupTargetAndIdempotency(t *testing.T) {
	// TC-002: 群聊目标映射为 openConversationId，uuid 与表情字段按 MCP 字段透传。
	caller := &personalEmotionCaller{}
	err := executePersonalEmotionCommand(t, caller,
		"emotion", "send",
		"--media-id", "@media",
		"--emotion-id", "emotion123",
		"--group", "cid123",
		"--idempotency-key", "idem-001",
	)
	if err != nil {
		t.Fatalf("chat emotion send returned error: %v", err)
	}
	requirePersonalEmotionCall(t, caller, "send_personal_emotion", map[string]any{
		"mediaId":            "@media",
		"emotionId":          "emotion123",
		"openConversationId": "cid123",
		"uuid":               "idem-001",
	})
}

func TestChatEmotionSendMapsOpenDingTalkTarget(t *testing.T) {
	// TC-003: 已知 openDingTalkId 时直传 receiverOpenDingTalkId，不做外部解析。
	caller := &personalEmotionCaller{}
	err := executePersonalEmotionCommand(t, caller,
		"emotion", "send",
		"--media-id", "@media",
		"--open-dingtalk-id", helperCurrentDOpenID,
	)
	if err != nil {
		t.Fatalf("chat emotion send returned error: %v", err)
	}
	requirePersonalEmotionCall(t, caller, "send_personal_emotion", map[string]any{
		"mediaId":                "@media",
		"receiverOpenDingTalkId": helperCurrentDOpenID,
	})
}

func TestChatEmotionSendTreatsOpenDingTalkIDPassedAsUserAsResolvedTarget(t *testing.T) {
	// TC-004: --user 收到 openDingTalkId 形态时保持 chat message send 的兼容语义。
	caller := &personalEmotionCaller{}
	err := executePersonalEmotionCommand(t, caller,
		"emotion", "send",
		"--media-id", "@media",
		"--user", helperCurrentDOpenID,
	)
	if err != nil {
		t.Fatalf("chat emotion send returned error: %v", err)
	}
	requirePersonalEmotionCall(t, caller, "send_personal_emotion", map[string]any{
		"mediaId":                "@media",
		"receiverOpenDingTalkId": helperCurrentDOpenID,
	})
}

func TestChatEmotionSendResolvesUserIDTarget(t *testing.T) {
	// TC-004b: --user 收到普通 userId 时先解析为 openDingTalkId，再发送个人收藏表情。
	caller := &personalEmotionCaller{}
	err := executePersonalEmotionCommand(t, caller,
		"emotion", "send",
		"--media-id", "@media",
		"--user", "u1",
	)
	if err != nil {
		t.Fatalf("chat emotion send returned error: %v", err)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("calls = %d, want contact resolve then send: %+v", len(caller.calls), caller.calls)
	}
	resolveCall := caller.calls[0]
	if resolveCall.server != "contact" || resolveCall.tool != "get_user_info_by_user_ids" {
		t.Fatalf("resolve call = %s/%s, want contact/get_user_info_by_user_ids", resolveCall.server, resolveCall.tool)
	}
	sendCall := caller.calls[1]
	if sendCall.server != "im" || sendCall.tool != "send_personal_emotion" {
		t.Fatalf("send call = %s/%s, want im/send_personal_emotion", sendCall.server, sendCall.tool)
	}
	want := map[string]any{
		"mediaId":                "@media",
		"receiverOpenDingTalkId": helperCurrentDOpenID2,
	}
	if !reflect.DeepEqual(sendCall.args, want) {
		t.Fatalf("send args = %#v, want %#v", sendCall.args, want)
	}
}

func TestChatEmotionSendRejectsInvalidTargets(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing target",
			args:    []string{"emotion", "send", "--media-id", "@media"},
			wantErr: "specify exactly one",
		},
		{
			name:    "multiple targets",
			args:    []string{"emotion", "send", "--media-id", "@media", "--group", "cid", "--open-dingtalk-id", helperCurrentDOpenID},
			wantErr: "specify exactly one",
		},
		{
			name:    "missing media",
			args:    []string{"emotion", "send", "--group", "cid"},
			wantErr: "--media-id is required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller := &personalEmotionCaller{}
			err := executePersonalEmotionCommand(t, caller, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
			if len(caller.calls) != 0 {
				t.Fatalf("invalid command reached MCP: %+v", caller.calls)
			}
		})
	}
}

func TestChatEmotionFavoriteMapsOptionalSourcePair(t *testing.T) {
	// TC-005: 收藏来源字段成对出现时透传为 sourceConversationId/sourceMessageId。
	caller := &personalEmotionCaller{}
	err := executePersonalEmotionCommand(t, caller,
		"emotion", "favorite",
		"--media-id", "@media",
		"--name", "赞",
		"--source-conversation-id", "cid123",
		"--source-message-id", "msg123",
	)
	if err != nil {
		t.Fatalf("chat emotion favorite returned error: %v", err)
	}
	requirePersonalEmotionCall(t, caller, "favorite_personal_emotion", map[string]any{
		"mediaId":              "@media",
		"name":                 "赞",
		"sourceConversationId": "cid123",
		"sourceMessageId":      "msg123",
	})
}

func TestChatEmotionFavoriteRejectsMissingRequiredOrUnpairedSource(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing media",
			args:    []string{"emotion", "favorite", "--name", "赞"},
			wantErr: "one of --media-id or --file-path is required",
		},
		{
			name:    "source conversation only",
			args:    []string{"emotion", "favorite", "--media-id", "@media", "--source-conversation-id", "cid123"},
			wantErr: "must be specified together",
		},
		{
			name:    "source message only",
			args:    []string{"emotion", "favorite", "--media-id", "@media", "--source-message-id", "msg123"},
			wantErr: "must be specified together",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller := &personalEmotionCaller{}
			err := executePersonalEmotionCommand(t, caller, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
			if len(caller.calls) != 0 {
				t.Fatalf("invalid command reached MCP: %+v", caller.calls)
			}
		})
	}
}

type personalEmotionUploadCaller struct {
	personalEmotionCaller
	uploadText    string
	uploadErr     error
	favoriteErr   error
	favoriteCalls []map[string]any
	uploadArgs    []map[string]any
	uploadCalls   int
	uploadServer  string
	uploadTool    string
}

func (c *personalEmotionUploadCaller) CallTool(ctx context.Context, server, tool string, args map[string]any) (*edition.ToolResult, error) {
	if server == personalEmotionUploadServerID && tool == personalEmotionUploadMediaTool {
		c.uploadCalls++
		c.uploadServer = server
		c.uploadTool = tool
		copied := make(map[string]any, len(args))
		for key, value := range args {
			copied[key] = value
		}
		c.uploadArgs = append(c.uploadArgs, copied)
		if c.uploadErr != nil {
			return nil, c.uploadErr
		}
		return textToolResult(c.uploadText), nil
	}
	if server == "im" && tool == "favorite_personal_emotion" {
		copied := make(map[string]any, len(args))
		for key, value := range args {
			copied[key] = value
		}
		c.favoriteCalls = append(c.favoriteCalls, copied)
		if c.favoriteErr != nil {
			return nil, c.favoriteErr
		}
		return textToolResult(`{"ok":true}`), nil
	}
	return c.personalEmotionCaller.CallTool(ctx, server, tool, args)
}

func writePersonalEmotionTestImage(t *testing.T, name string, size int) string {
	t.Helper()
	filePath := filepath.Join(t.TempDir(), name)
	payload := make([]byte, size)
	if err := os.WriteFile(filePath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return filePath
}

func writeLargePersonalEmotionPNG(t *testing.T) (string, []byte) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1600, 1600))
	seed := uint32(1)
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			seed = seed*1664525 + 1013904223
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(seed >> 24),
				G: uint8(seed >> 16),
				B: uint8(seed >> 8),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if buf.Len() <= int(personalEmotionImageMaxBytes) {
		t.Fatalf("test PNG size = %d, want > %d", buf.Len(), personalEmotionImageMaxBytes)
	}
	filePath := filepath.Join(t.TempDir(), "large.png")
	if err := os.WriteFile(filePath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return filePath, buf.Bytes()
}

func writeLargePersonalEmotionGIF(t *testing.T) (string, []byte) {
	t.Helper()
	palette := make(color.Palette, 0, 256)
	for i := 0; i < 256; i++ {
		palette = append(palette, color.RGBA{R: uint8(i), G: uint8(255 - i), B: uint8((i * 37) % 256), A: 255})
	}
	anim := &gif.GIF{LoopCount: 0}
	seed := uint32(7)
	for frameIndex := 0; frameIndex < 30; frameIndex++ {
		frame := image.NewPaletted(image.Rect(0, 0, 360, 360), palette)
		for y := 0; y < 360; y++ {
			for x := 0; x < 360; x++ {
				seed = seed*1664525 + 1013904223
				frame.SetColorIndex(x, y, uint8(seed>>24))
			}
		}
		anim.Image = append(anim.Image, frame)
		anim.Delay = append(anim.Delay, 8)
		anim.Disposal = append(anim.Disposal, gif.DisposalBackground)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, anim); err != nil {
		t.Fatal(err)
	}
	if buf.Len() <= int(personalEmotionImageMaxBytes) {
		t.Fatalf("test GIF size = %d, want > %d", buf.Len(), personalEmotionImageMaxBytes)
	}
	filePath := filepath.Join(t.TempDir(), "large.gif")
	if err := os.WriteFile(filePath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return filePath, buf.Bytes()
}

func executePersonalEmotionCallerCommand(t *testing.T, caller edition.ToolCaller, args ...string) error {
	t.Helper()
	installHelpersCoreDeps(t, caller)
	root := newChatCommand()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	return root.ExecuteContext(context.Background())
}

func TestChatEmotionFavoriteFilePathUploadsThenFavorites(t *testing.T) {
	// AC-01: --file-path 先经钉钉文件服务 upload_media (chat_image) 取 mediaId，再复用收藏链路。
	imagePath := writePersonalEmotionTestImage(t, "sticker.png", 16)
	caller := &personalEmotionUploadCaller{uploadText: `{"success":true,"logId":"log-1","mediaIdV2":"$v2-media","mediaIdV2Url":"https://down.dingtalk.com/ddmedia/v2.jpg","message":"图片上传成功。","imageType":"png","bizType":"chat_image"}`}
	err := executePersonalEmotionCallerCommand(t, caller,
		"emotion", "favorite",
		"--file-path", imagePath,
		"--name", "本地表情",
	)
	if err != nil {
		t.Fatalf("chat emotion favorite --file-path returned error: %v", err)
	}
	if caller.uploadCalls != 1 {
		t.Fatalf("upload calls = %d, want 1", caller.uploadCalls)
	}
	if caller.uploadServer != personalEmotionUploadServerID || caller.uploadTool != personalEmotionUploadMediaTool {
		t.Fatalf("upload target = %s/%s, want %s/%s", caller.uploadServer, caller.uploadTool, personalEmotionUploadServerID, personalEmotionUploadMediaTool)
	}
	imageData, readErr := os.ReadFile(imagePath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	wantUpload := map[string]any{
		"content":   base64.StdEncoding.EncodeToString(imageData),
		"imageType": "png",
		"bizType":   "chat_image",
	}
	if !reflect.DeepEqual(caller.uploadArgs[0], wantUpload) {
		t.Fatalf("upload args = %#v, want %#v", caller.uploadArgs[0], wantUpload)
	}
	if len(caller.favoriteCalls) != 1 {
		t.Fatalf("favorite calls = %d, want 1", len(caller.favoriteCalls))
	}
	want := map[string]any{"mediaId": "$v2-media", "name": "本地表情"}
	if !reflect.DeepEqual(caller.favoriteCalls[0], want) {
		t.Fatalf("favorite args = %#v, want %#v", caller.favoriteCalls[0], want)
	}
}

func TestChatEmotionFavoriteFilePathCompressesLargePNGBeforeUpload(t *testing.T) {
	// AC-01b: 超过 2MB 的静态图先本地压缩，再以压缩后的 jpg 内容上传。
	imagePath, original := writeLargePersonalEmotionPNG(t)
	caller := &personalEmotionUploadCaller{uploadText: `{"success":true,"mediaIdV2":"$v2-media"}`}
	err := executePersonalEmotionCallerCommand(t, caller, "emotion", "favorite", "--file-path", imagePath)
	if err != nil {
		t.Fatalf("large png favorite returned error: %v", err)
	}
	if caller.uploadCalls != 1 || len(caller.favoriteCalls) != 1 {
		t.Fatalf("calls: upload=%d favorite=%d", caller.uploadCalls, len(caller.favoriteCalls))
	}
	if got := caller.uploadArgs[0]["imageType"]; got != "jpg" {
		t.Fatalf("compressed imageType = %v, want jpg", got)
	}
	content, err := base64.StdEncoding.DecodeString(caller.uploadArgs[0]["content"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if len(content) > int(personalEmotionImageMaxBytes) || len(content) >= len(original) {
		t.Fatalf("compressed size = %d original = %d limit = %d", len(content), len(original), personalEmotionImageMaxBytes)
	}
	if _, err := jpeg.Decode(bytes.NewReader(content)); err != nil {
		t.Fatalf("compressed content is not jpeg: %v", err)
	}
}

func TestChatEmotionFavoriteFilePathCompressesLargeGIFKeepingAnimation(t *testing.T) {
	// AC-01c: 超过 2MB 的 GIF 压缩后仍以 gif 上传，并保留多帧动图。
	imagePath, original := writeLargePersonalEmotionGIF(t)
	caller := &personalEmotionUploadCaller{uploadText: `{"success":true,"mediaIdV2":"$v2-media"}`}
	err := executePersonalEmotionCallerCommand(t, caller, "emotion", "favorite", "--file-path", imagePath)
	if err != nil {
		t.Fatalf("large gif favorite returned error: %v", err)
	}
	if caller.uploadCalls != 1 || len(caller.favoriteCalls) != 1 {
		t.Fatalf("calls: upload=%d favorite=%d", caller.uploadCalls, len(caller.favoriteCalls))
	}
	if got := caller.uploadArgs[0]["imageType"]; got != "gif" {
		t.Fatalf("compressed imageType = %v, want gif", got)
	}
	content, err := base64.StdEncoding.DecodeString(caller.uploadArgs[0]["content"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if len(content) > int(personalEmotionImageMaxBytes) || len(content) >= len(original) {
		t.Fatalf("compressed size = %d original = %d limit = %d", len(content), len(original), personalEmotionImageMaxBytes)
	}
	anim, err := gif.DecodeAll(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("compressed content is not gif: %v", err)
	}
	if len(anim.Image) < 2 {
		t.Fatalf("compressed gif frames = %d, want animated", len(anim.Image))
	}
}

func TestChatEmotionFavoriteFilePathMutuallyExclusiveWithMediaID(t *testing.T) {
	// E-02: --media-id 与 --file-path 同时传被 Cobra 互斥拦截，不产生任何 MCP 调用。
	imagePath := writePersonalEmotionTestImage(t, "sticker.png", 8)
	caller := &personalEmotionUploadCaller{uploadText: `{"success":true,"mediaIdV1":"@v1"}`}
	err := executePersonalEmotionCallerCommand(t, caller,
		"emotion", "favorite",
		"--media-id", "@media",
		"--file-path", imagePath,
	)
	if err == nil || !strings.Contains(err.Error(), "none of the others can be") {
		t.Fatalf("error = %v, want mutually exclusive rejection", err)
	}
	if caller.uploadCalls != 0 || len(caller.favoriteCalls) != 0 {
		t.Fatalf("mutually exclusive argv reached MCP: upload=%d favorite=%d", caller.uploadCalls, len(caller.favoriteCalls))
	}
}

func TestChatEmotionFavoriteFilePathRejectsInvalidLocalFiles(t *testing.T) {
	// E-03/E-04/E-05/E-06: 本地校验全部在 0 次 MCP 调用前拦截。
	oversizePath := filepath.Join(t.TempDir(), "large.png")
	if err := os.WriteFile(oversizePath, make([]byte, personalEmotionImageMaxBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	tooLargePath := filepath.Join(t.TempDir(), "too-large.png")
	if err := os.WriteFile(tooLargePath, make([]byte, personalEmotionImageAutoCompressBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "missing file", path: filepath.Join(t.TempDir(), "absent.png"), wantErr: "cannot read local image"},
		{name: "directory", path: t.TempDir(), wantErr: "directory"},
		{name: "oversize decode failure", path: oversizePath, wantErr: "automatic compression failed"},
		{name: "too large for automatic compression", path: tooLargePath, wantErr: "exceeds the 10MB automatic compression limit"},
		{name: "unsupported extension", path: writePersonalEmotionTestImage(t, "pic.tiff", 4), wantErr: "only supports"},
		{name: "no extension", path: writePersonalEmotionTestImage(t, "plainfile", 4), wantErr: "only supports"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller := &personalEmotionUploadCaller{}
			err := executePersonalEmotionCallerCommand(t, caller, "emotion", "favorite", "--file-path", tc.path)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
			if caller.uploadCalls != 0 || len(caller.favoriteCalls) != 0 {
				t.Fatalf("invalid file reached MCP: upload=%d favorite=%d", caller.uploadCalls, len(caller.favoriteCalls))
			}
		})
	}
}

func TestChatEmotionFavoriteFilePathAcceptsUppercaseExtension(t *testing.T) {
	// AC-02: 大写扩展名归一化后正常进入上传链路。
	upperPath := filepath.Join(t.TempDir(), "Sticker.PNG")
	if err := os.WriteFile(upperPath, []byte("png payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	caller := &personalEmotionUploadCaller{uploadText: `{"success":true,"mediaIdV1":"@v1"}`}
	if err := executePersonalEmotionCallerCommand(t, caller, "emotion", "favorite", "--file-path", upperPath); err != nil {
		t.Fatalf("uppercase extension rejected: %v", err)
	}
	if caller.uploadCalls != 1 || len(caller.favoriteCalls) != 1 {
		t.Fatalf("calls: upload=%d favorite=%d", caller.uploadCalls, len(caller.favoriteCalls))
	}
	if got := caller.uploadArgs[0]["imageType"]; got != "png" {
		t.Fatalf("upload imageType = %v, want normalized png", got)
	}
}

func TestChatEmotionFavoriteFilePathRejectsUnpairedSource(t *testing.T) {
	// E-07: source 成对校验对 file-path 路径同样生效（0 次 MCP 调用）。
	imagePath := writePersonalEmotionTestImage(t, "sticker.png", 8)
	caller := &personalEmotionUploadCaller{}
	err := executePersonalEmotionCallerCommand(t, caller,
		"emotion", "favorite",
		"--file-path", imagePath,
		"--source-conversation-id", "cid123",
	)
	if err == nil || !strings.Contains(err.Error(), "must be specified together") {
		t.Fatalf("error = %v, want unpaired source rejection", err)
	}
	if caller.uploadCalls != 0 {
		t.Fatalf("unpaired source reached MCP: upload=%d", caller.uploadCalls)
	}
}

func TestChatEmotionFavoriteFilePathUploadFailureSurfacesServerError(t *testing.T) {
	// E-08: upload success=false 透传 errorCode/errorMsg/logId，不执行收藏。
	imagePath := writePersonalEmotionTestImage(t, "sticker.png", 8)
	caller := &personalEmotionUploadCaller{
		uploadText: `{"success":false,"logId":"log-9","errorCode":"invalidRequest.contentTooLarge","errorMsg":"decoded content exceeds 10MB"}`,
	}
	err := executePersonalEmotionCallerCommand(t, caller, "emotion", "favorite", "--file-path", imagePath)
	if err == nil {
		t.Fatal("upload failure not surfaced")
	}
	for _, want := range []string{"invalidRequest.contentTooLarge", "decoded content exceeds 10MB", "log-9"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
	if caller.uploadCalls != 1 || len(caller.favoriteCalls) != 0 {
		t.Fatalf("calls after upload failure: upload=%d favorite=%d", caller.uploadCalls, len(caller.favoriteCalls))
	}
}

func TestChatEmotionFavoriteFilePathFallsBackToMediaIDV2(t *testing.T) {
	// C-01: 新文件服务只返回 mediaIdV2 时，直接使用 V2 mediaId 进入收藏。
	imagePath := writePersonalEmotionTestImage(t, "sticker.png", 8)
	caller := &personalEmotionUploadCaller{
		uploadText: `{"success":true,"mediaIdV2":"$v2$","mediaIdV1":"","logId":"log-2"}`,
	}
	err := executePersonalEmotionCallerCommand(t, caller, "emotion", "favorite", "--file-path", imagePath)
	if err != nil {
		t.Fatalf("mediaIdV2 fallback failed: %v", err)
	}
	if caller.uploadCalls != 1 || len(caller.favoriteCalls) != 1 {
		t.Fatalf("calls after V2 fallback: upload=%d favorite=%d", caller.uploadCalls, len(caller.favoriteCalls))
	}
	want := map[string]any{"mediaId": "$v2$"}
	if !reflect.DeepEqual(caller.favoriteCalls[0], want) {
		t.Fatalf("favorite args = %#v, want %#v", caller.favoriteCalls[0], want)
	}
}

func TestChatEmotionFavoriteFilePathWrapsFavoriteFailureWithUploadedMediaID(t *testing.T) {
	// E-09: favorite 失败时附上已上传 mediaId，提示用 --media-id 重试。
	imagePath := writePersonalEmotionTestImage(t, "sticker.png", 8)
	caller := &personalEmotionUploadCaller{
		uploadText:  `{"success":true,"mediaIdV1":"@v1-media"}`,
		favoriteErr: fmt.Errorf("boom from favorite"),
	}
	err := executePersonalEmotionCallerCommand(t, caller, "emotion", "favorite", "--file-path", imagePath)
	if err == nil {
		t.Fatal("favorite failure not surfaced")
	}
	for _, want := range []string{"@v1-media", "--media-id", "boom from favorite"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
	if caller.uploadCalls != 1 || len(caller.favoriteCalls) != 1 {
		t.Fatalf("calls: upload=%d favorite=%d", caller.uploadCalls, len(caller.favoriteCalls))
	}
}

type dryRunPersonalEmotionCaller struct {
	delegate *personalEmotionUploadCaller
}

func (c *dryRunPersonalEmotionCaller) CallTool(ctx context.Context, server, tool string, args map[string]any) (*edition.ToolResult, error) {
	return c.delegate.CallTool(ctx, server, tool, args)
}
func (c *dryRunPersonalEmotionCaller) Format() string { return "json" }
func (c *dryRunPersonalEmotionCaller) DryRun() bool   { return true }
func (c *dryRunPersonalEmotionCaller) Fields() string { return "" }
func (c *dryRunPersonalEmotionCaller) JQ() string     { return "" }

func TestChatEmotionFavoriteFilePathDryRunStaysLocal(t *testing.T) {
	// AC-06: --dry-run 仅本地校验并输出计划，0 次 MCP 调用。
	imagePath := writePersonalEmotionTestImage(t, "sticker.png", 12)
	caller := &personalEmotionUploadCaller{uploadText: `{"success":true,"mediaIdV1":"@v1"}`}
	installHelpersCoreDeps(t, &dryRunPersonalEmotionCaller{delegate: caller})
	root := newChatCommand()
	root.SilenceErrors = true
	root.SilenceUsage = true
	if root.PersistentFlags().Lookup("dry-run") == nil {
		root.PersistentFlags().Bool("dry-run", false, "preview without executing")
	}
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"emotion", "favorite", "--file-path", imagePath, "--dry-run"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	if caller.uploadCalls != 0 || len(caller.favoriteCalls) != 0 {
		t.Fatalf("dry-run reached MCP: upload=%d favorite=%d", caller.uploadCalls, len(caller.favoriteCalls))
	}
}

func TestPersonalEmotionImageFileValidation(t *testing.T) {
	// T1 纯函数表测：扩展名映射（含大小写归一、jpg/jpeg 区分）与 stat 级校验。
	for ext, want := range map[string]string{
		".jpg": "jpg", ".jpeg": "jpeg", ".png": "png",
		".gif": "gif", ".webp": "webp", ".bmp": "bmp",
	} {
		if got, ok := personalEmotionImageType(ext); !ok || got != want {
			t.Fatalf("personalEmotionImageType(%q) = %q,%v want %q,true", ext, got, ok, want)
		}
	}
	for _, ext := range []string{".JPG", ".PNG", ".WebP"} {
		if _, ok := personalEmotionImageType(ext); !ok {
			t.Fatalf("personalEmotionImageType(%q) rejected", ext)
		}
	}
	for _, ext := range []string{".tiff", "", ".txt"} {
		if _, ok := personalEmotionImageType(ext); ok {
			t.Fatalf("personalEmotionImageType(%q) unexpectedly accepted", ext)
		}
	}

	dir := t.TempDir()
	validPath := filepath.Join(dir, "ok.png")
	if err := os.WriteFile(validPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, imageType, err := validatePersonalEmotionImageFile(validPath); err != nil || imageType != "png" {
		t.Fatalf("valid file: imageType=%q err=%v", imageType, err)
	}
	if _, _, err := validatePersonalEmotionImageFile(filepath.Join(dir, "absent.png")); err == nil {
		t.Fatal("absent file accepted")
	}
	if _, _, err := validatePersonalEmotionImageFile(dir); err == nil {
		t.Fatal("directory accepted")
	}
	overPath := filepath.Join(dir, "big.png")
	if err := os.WriteFile(overPath, make([]byte, personalEmotionImageMaxBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if size, imageType, err := validatePersonalEmotionImageFile(overPath); err != nil || size != personalEmotionImageMaxBytes+1 || imageType != "png" {
		t.Fatalf("oversize extension validation: size=%d imageType=%q err=%v", size, imageType, err)
	}
	boundaryPath := filepath.Join(dir, "exact.png")
	if err := os.WriteFile(boundaryPath, make([]byte, personalEmotionImageMaxBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	if size, imageType, err := validatePersonalEmotionImageFile(boundaryPath); err != nil || size != personalEmotionImageMaxBytes || imageType != "png" {
		t.Fatalf("boundary file: size=%d imageType=%q err=%v", size, imageType, err)
	}
	if _, _, err := validatePersonalEmotionImageFile(filepath.Join(dir, "bad.tiff")); err == nil {
		t.Fatal("unsupported extension accepted")
	}
}

func TestPersonalEmotionUploadMediaIDParsing(t *testing.T) {
	// 解析层表测：success=false 透传错误字段；V1 优先；V1 缺失时兼容新文件服务 mediaIdV2。
	if got, err := parsePersonalEmotionUploadMediaID(`{"success":true,"mediaIdV1":"@v1","mediaIdV2":"$v2$"}`); err != nil || got != "@v1" {
		t.Fatalf("happy path = %q, %v", got, err)
	}
	if got, err := parsePersonalEmotionUploadMediaID(`{"bizType":"chat_image","mediaIdV2":"$iwElAqNqcGcDAQTR","mediaIdV2Url":"https://down.dingtalk.com/ddmedia/iwElAqNqcGcDAQTR.jpg","success":true,"logId":"2103f43517878108447396157e0854","message":"图片上传成功。","imageType":"jpg"}`); err != nil || got != "$iwElAqNqcGcDAQTR" {
		t.Fatalf("mediaIdV2 path = %q, %v", got, err)
	}
	_, err := parsePersonalEmotionUploadMediaID(`{"success":false,"errorCode":"internalError","errorMsg":"uploadAuthFile failed","logId":"log-3"}`)
	if err == nil {
		t.Fatal("failure payload accepted")
	}
	for _, want := range []string{"internalError", "uploadAuthFile failed", "log-3"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
	if _, err := parsePersonalEmotionUploadMediaID(`{"success":true}`); err == nil {
		t.Fatal("missing mediaId accepted")
	}
	if _, err := parsePersonalEmotionUploadMediaID(`not-json`); err == nil {
		t.Fatal("non-JSON accepted")
	}
}
