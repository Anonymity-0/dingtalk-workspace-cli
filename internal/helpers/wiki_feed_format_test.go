package helpers

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// ---------- formatFeedTime unit tests ----------

func TestCrossPlatformCoverageFormatFeedTimeInvalidJSON(t *testing.T) {
	// JSON 解析失败 → 原样返回原始字符串
	raw := "not-json"
	got := formatFeedTime(raw)
	s, ok := got.(string)
	if !ok || s != raw {
		t.Fatalf("formatFeedTime(invalid) = %v (%T), want raw string %q", got, got, raw)
	}
}

func TestCrossPlatformCoverageFormatFeedTimeEmptyInput(t *testing.T) {
	// 空字符串 → JSON 解析失败 → 原样返回
	got := formatFeedTime("")
	s, ok := got.(string)
	if !ok || s != "" {
		t.Fatalf("formatFeedTime(\"\") = %v (%T), want \"\"", got, got)
	}
}

func TestCrossPlatformCoverageFormatFeedTimeNoFeedsKey(t *testing.T) {
	// 合法 JSON 但没有 feeds 字段 → 返回解析后的 map
	raw := `{"status":"ok","count":0}`
	got := formatFeedTime(raw)
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("formatFeedTime(no feeds) = %T, want map[string]any", got)
	}
	if m["status"] != "ok" {
		t.Fatalf("status = %v, want ok", m["status"])
	}
}

func TestCrossPlatformCoverageFormatFeedTimeFeedsNotArray(t *testing.T) {
	// feeds 不是数组 → 返回解析后的 map
	raw := `{"feeds":"unexpected-string"}`
	got := formatFeedTime(raw)
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("formatFeedTime(feeds string) = %T, want map[string]any", got)
	}
	if m["feeds"] != "unexpected-string" {
		t.Fatalf("feeds = %v, want unexpected-string", m["feeds"])
	}
}

func TestCrossPlatformCoverageFormatFeedTimeFeedsNull(t *testing.T) {
	// feeds 为 null → 类型断言失败 → 返回解析后的 map
	raw := `{"feeds":null}`
	got := formatFeedTime(raw)
	_, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("formatFeedTime(feeds null) = %T, want map[string]any", got)
	}
}

func TestCrossPlatformCoverageFormatFeedTimeNonMapItem(t *testing.T) {
	// feeds 数组元素不是 object → continue 跳过
	raw := `{"feeds":["string-item", 42, null]}`
	got := formatFeedTime(raw)
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	feeds := m["feeds"].([]any)
	if len(feeds) != 3 {
		t.Fatalf("feeds length = %d, want 3", len(feeds))
	}
}

func TestCrossPlatformCoverageFormatFeedTimeTimeMissingOrInvalid(t *testing.T) {
	// time 字段缺失或不是 float64 或 <= 0 → 跳过格式化
	raw := `{"feeds":[
		{"id":"a"},
		{"id":"b","time":"not-a-number"},
		{"id":"c","time":0},
		{"id":"d","time":-1000}
	]}`
	got := formatFeedTime(raw)
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	feeds := m["feeds"].([]any)
	for i, f := range feeds {
		item := f.(map[string]any)
		if _, hasTimeMs := item["timeMs"]; hasTimeMs {
			t.Fatalf("feed[%d] should not have timeMs, got %v", i, item)
		}
	}
}

func TestCrossPlatformCoverageFormatFeedTimeNormalPath(t *testing.T) {
	// 正常路径：毫秒时间戳被格式化，原始值保留到 timeMs
	// 2025-06-15 10:30:00 UTC = 2025-06-15 18:30:00 CST (UTC+8)
	tsMs := float64(1750067400000)
	raw := `{"feeds":[{"time":` + json.Number(strings.TrimRight(strings.TrimRight(
		strings.Replace(
			string(mustMarshal(t, map[string]any{"v": tsMs})),
			`{"v":`, "", 1),
		"}"), "")).String() + `,"type":1,"id":"f1"}]}`
	// Build a cleaner JSON payload
	raw = buildFeedJSON(t, []map[string]any{
		{"time": tsMs, "type": float64(1), "id": "f1"},
	})

	got := formatFeedTime(raw)
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	feeds := m["feeds"].([]any)
	if len(feeds) != 1 {
		t.Fatalf("feeds length = %d, want 1", len(feeds))
	}
	item := feeds[0].(map[string]any)

	// timeMs 保留原始毫秒值
	if ms, ok := item["timeMs"].(float64); !ok || ms != tsMs {
		t.Fatalf("timeMs = %v, want %v", item["timeMs"], tsMs)
	}

	// time 被格式化为北京时间字符串
	timeStr, ok := item["time"].(string)
	if !ok {
		t.Fatalf("time = %T, want string", item["time"])
	}
	expected := time.UnixMilli(int64(tsMs)).In(beijingLoc).Format("2006-01-02 15:04")
	if timeStr != expected {
		t.Fatalf("time = %q, want %q", timeStr, expected)
	}

	// typeLabel 被填充
	if item["typeLabel"] != "更新文档" {
		t.Fatalf("typeLabel = %v, want 更新文档", item["typeLabel"])
	}
}

func TestCrossPlatformCoverageFormatFeedTimeMultipleFeeds(t *testing.T) {
	// 多个 feed，混合有效/无效时间戳
	ts1 := float64(1750067400000)
	ts2 := float64(1700000000000)
	raw := buildFeedJSON(t, []map[string]any{
		{"time": ts1, "type": float64(0), "id": "f1"},
		{"time": ts2, "type": float64(12), "id": "f2"},
		{"id": "f3"}, // 没有 time → 跳过
	})

	got := formatFeedTime(raw)
	m := got.(map[string]any)
	feeds := m["feeds"].([]any)

	// 前两个有时间戳
	for i := 0; i < 2; i++ {
		item := feeds[i].(map[string]any)
		if _, ok := item["timeMs"].(float64); !ok {
			t.Fatalf("feed[%d] missing timeMs", i)
		}
		if _, ok := item["time"].(string); !ok {
			t.Fatalf("feed[%d] time not string", i)
		}
	}

	// 第三个没有时间戳
	item3 := feeds[2].(map[string]any)
	if _, hasTimeMs := item3["timeMs"]; hasTimeMs {
		t.Fatalf("feed[2] should not have timeMs")
	}
}

// ---------- trimFeedFields unit tests ----------

func TestCrossPlatformCoverageTrimFeedFieldsRemovesParentDocAndUserInfo(t *testing.T) {
	item := map[string]any{
		"parentDoc": map[string]any{"name": "parent"},
		"userInfo":  map[string]any{"nick": "user1"},
		"id":        "f1",
	}
	trimFeedFields(item)
	if _, ok := item["parentDoc"]; ok {
		t.Fatal("parentDoc should be removed")
	}
	if _, ok := item["userInfo"]; ok {
		t.Fatal("userInfo should be removed")
	}
	if item["id"] != "f1" {
		t.Fatal("id should be preserved")
	}
}

func TestCrossPlatformCoverageTrimFeedFieldsTypeLabelKnown(t *testing.T) {
	// 已知 type → 添加 typeLabel
	for typeNum, expectedLabel := range feedTypeLabels {
		item := map[string]any{"type": float64(typeNum)}
		trimFeedFields(item)
		if item["typeLabel"] != expectedLabel {
			t.Fatalf("type %d: typeLabel = %v, want %q", typeNum, item["typeLabel"], expectedLabel)
		}
	}
}

func TestCrossPlatformCoverageTrimFeedFieldsTypeLabelUnknown(t *testing.T) {
	// 未知 type → 不添加 typeLabel
	item := map[string]any{"type": float64(999)}
	trimFeedFields(item)
	if _, ok := item["typeLabel"]; ok {
		t.Fatal("typeLabel should not be set for unknown type 999")
	}
}

func TestCrossPlatformCoverageTrimFeedFieldsTypeNotFloat(t *testing.T) {
	// type 不是 float64 → 不添加 typeLabel
	item := map[string]any{"type": "string-type"}
	trimFeedFields(item)
	if _, ok := item["typeLabel"]; ok {
		t.Fatal("typeLabel should not be set for non-float type")
	}
}

func TestCrossPlatformCoverageTrimFeedFieldsUsersTrim(t *testing.T) {
	// users 数组中的 user 对象只保留 nick
	item := map[string]any{
		"users": []any{
			map[string]any{"nick": "Alice", "avatarUrl": "http://img", "userId": "u1"},
			map[string]any{"nick": "Bob", "email": "bob@test"},
		},
	}
	trimFeedFields(item)
	users := item["users"].([]any)
	for i, u := range users {
		user := u.(map[string]any)
		if _, ok := user["nick"]; !ok {
			t.Fatalf("users[%d] should have nick", i)
		}
		for k := range user {
			if k != "nick" {
				t.Fatalf("users[%d] unexpected field %q", i, k)
			}
		}
	}
	if users[0].(map[string]any)["nick"] != "Alice" {
		t.Fatalf("users[0].nick = %v, want Alice", users[0].(map[string]any)["nick"])
	}
}

func TestCrossPlatformCoverageTrimFeedFieldsUsersNilNick(t *testing.T) {
	// users 中 nick 为 nil → 删除后不回填
	item := map[string]any{
		"users": []any{
			map[string]any{"userId": "u1"},
		},
	}
	trimFeedFields(item)
	user := item["users"].([]any)[0].(map[string]any)
	if len(user) != 0 {
		t.Fatalf("user should be empty after trimming nil nick, got %v", user)
	}
}

func TestCrossPlatformCoverageTrimFeedFieldsUsersNonMapItem(t *testing.T) {
	// users 数组中有非 map 元素 → 不 panic
	item := map[string]any{
		"users": []any{"not-a-map", 42, nil},
	}
	trimFeedFields(item) // 不应 panic
	users := item["users"].([]any)
	if len(users) != 3 {
		t.Fatalf("users length changed: %d", len(users))
	}
}

func TestCrossPlatformCoverageTrimFeedFieldsContentDocTrim(t *testing.T) {
	// content.doc 只保留 name 和 extension
	item := map[string]any{
		"content": map[string]any{
			"doc": map[string]any{
				"name":      "测试文档",
				"extension": "adoc",
				"docKey":    "dk123",
				"url":       "http://example",
			},
			"dentryKey":    "entry1",
			"workspaceKey": "ws1",
		},
	}
	trimFeedFields(item)
	content := item["content"].(map[string]any)
	// content 中只保留 doc
	for k := range content {
		if k != "doc" {
			t.Fatalf("content should only have doc, found %q", k)
		}
	}
	doc := content["doc"].(map[string]any)
	if doc["name"] != "测试文档" {
		t.Fatalf("doc.name = %v, want 测试文档", doc["name"])
	}
	if doc["extension"] != "adoc" {
		t.Fatalf("doc.extension = %v, want adoc", doc["extension"])
	}
	for k := range doc {
		if k != "name" && k != "extension" {
			t.Fatalf("doc unexpected field %q", k)
		}
	}
}

func TestCrossPlatformCoverageTrimFeedFieldsContentDocNilFields(t *testing.T) {
	// content.doc 中 name 和 extension 缺失 → 不回填
	item := map[string]any{
		"content": map[string]any{
			"doc": map[string]any{
				"docKey": "dk123",
			},
		},
	}
	trimFeedFields(item)
	doc := item["content"].(map[string]any)["doc"].(map[string]any)
	if len(doc) != 0 {
		t.Fatalf("doc should be empty, got %v", doc)
	}
}

func TestCrossPlatformCoverageTrimFeedFieldsContentNoDoc(t *testing.T) {
	// content 存在但没有 doc 子字段 → 移除其他字段
	item := map[string]any{
		"content": map[string]any{
			"dentryKey":    "entry1",
			"workspaceKey": "ws1",
		},
	}
	trimFeedFields(item)
	content := item["content"].(map[string]any)
	if len(content) != 0 {
		t.Fatalf("content should be empty, got %v", content)
	}
}

func TestCrossPlatformCoverageTrimFeedFieldsNoContentNoUsers(t *testing.T) {
	// 没有 content / users / type → 仅删除 parentDoc / userInfo
	item := map[string]any{
		"id":        "f1",
		"parentDoc": map[string]any{"x": 1},
	}
	trimFeedFields(item)
	if _, ok := item["parentDoc"]; ok {
		t.Fatal("parentDoc should be removed")
	}
	if item["id"] != "f1" {
		t.Fatal("id should be preserved")
	}
}

func TestCrossPlatformCoverageTrimFeedFieldsEmptyItem(t *testing.T) {
	// 空 map → 不 panic
	item := map[string]any{}
	trimFeedFields(item)
	if len(item) != 0 {
		t.Fatalf("empty item changed: %v", item)
	}
}

// ---------- RunE integration: feed list with formatFeedTime ----------

func TestCrossPlatformCoverageWikiFeedListFormatIntegration(t *testing.T) {
	// 通过 scriptedToolCaller 返回真实结构的 feed JSON，验证 RunE 中
	// callMCPToolReturnText → formatFeedTime → PrintJSON 完整路径
	ts := float64(1750067400000)
	feedPayload := map[string]any{
		"feeds": []any{
			map[string]any{
				"time":      ts,
				"type":      float64(0),
				"id":        "feed1",
				"parentDoc": map[string]any{"name": "parent"},
				"userInfo":  map[string]any{"nick": "user1", "avatarUrl": "http://img"},
				"users": []any{
					map[string]any{"nick": "Alice", "userId": "u1", "avatarUrl": "http://avatar"},
				},
				"content": map[string]any{
					"doc": map[string]any{
						"name":      "测试文档",
						"extension": "adoc",
						"docKey":    "dk123",
					},
					"dentryKey":    "entry1",
					"workspaceKey": "ws1",
				},
			},
			map[string]any{
				"time": float64(1700000000000),
				"type": float64(7),
				"id":   "feed2",
			},
		},
	}
	payloadBytes, err := json.Marshal(feedPayload)
	if err != nil {
		t.Fatal(err)
	}

	caller := &scriptedToolCaller{
		steps: []scriptedToolStep{{text: string(payloadBytes)}},
	}

	var out bytes.Buffer
	oldDeps := deps
	oldArgs := os.Args
	InitDeps(caller)
	deps.Out.w = &out
	deps.Out.errW = io.Discard
	t.Cleanup(func() {
		deps = oldDeps
		os.Args = oldArgs
	})

	root := newWikiCommand()
	installExampleGlobalFlags(root)
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetIn(os.Stdin)

	args := []string{"feed", "list", "--workspace", "ws1", "--exclude-file"}
	root.SetArgs(args)
	os.Args = append([]string{"dws", "wiki"}, args...)

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// 验证输出是合法 JSON 且包含格式化后的 feed
	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	feeds, ok := parsed["feeds"].([]any)
	if !ok || len(feeds) != 2 {
		t.Fatalf("feeds length unexpected: %v", parsed["feeds"])
	}

	// 第一个 feed 验证 timeMs 保留、time 格式化、字段裁剪
	f0 := feeds[0].(map[string]any)
	if f0["timeMs"] != ts {
		t.Fatalf("timeMs = %v, want %v", f0["timeMs"], ts)
	}
	if _, ok := f0["time"].(string); !ok {
		t.Fatalf("time should be string, got %T", f0["time"])
	}
	if _, ok := f0["parentDoc"]; ok {
		t.Fatal("parentDoc should be trimmed")
	}
	if _, ok := f0["userInfo"]; ok {
		t.Fatal("userInfo should be trimmed")
	}
	if f0["typeLabel"] != "创建文档" {
		t.Fatalf("typeLabel = %v, want 创建文档", f0["typeLabel"])
	}

	// users 裁剪验证
	if users, ok := f0["users"].([]any); ok {
		for _, u := range users {
			um := u.(map[string]any)
			if _, ok := um["avatarUrl"]; ok {
				t.Fatal("user avatarUrl should be trimmed")
			}
		}
	}

	// content 裁剪验证
	if content, ok := f0["content"].(map[string]any); ok {
		if _, ok := content["dentryKey"]; ok {
			t.Fatal("content.dentryKey should be trimmed")
		}
	}
}

func TestCrossPlatformCoverageWikiFeedListEmptyResponse(t *testing.T) {
	// MCP 返回空结果 → formatFeedTime 处理空字符串 → 不报错
	caller := &scriptedToolCaller{} // 无 steps → 空结果

	var out bytes.Buffer
	oldDeps := deps
	oldArgs := os.Args
	InitDeps(caller)
	deps.Out.w = &out
	deps.Out.errW = io.Discard
	t.Cleanup(func() {
		deps = oldDeps
		os.Args = oldArgs
	})

	root := newWikiCommand()
	installExampleGlobalFlags(root)
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetIn(os.Stdin)

	args := []string{"feed", "list", "--workspace", "ws1"}
	root.SetArgs(args)
	os.Args = append([]string{"dws", "wiki"}, args...)

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

// ---------- helpers ----------

func buildFeedJSON(t *testing.T, feeds []map[string]any) string {
	t.Helper()
	payload := map[string]any{"feeds": feeds}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
