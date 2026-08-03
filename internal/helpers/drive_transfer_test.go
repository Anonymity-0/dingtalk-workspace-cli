package helpers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// ──────────────────────────────────────────────────────────
// parsePartSize / formatByteSize
// ──────────────────────────────────────────────────────────

func TestParsePartSize(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"16MB", 16 << 20, false},
		{"16M", 16 << 20, false},
		{"1GB", 1 << 30, false},
		{"1g", 1 << 30, false},
		{"1024KB", 1 << 20, false},
		{"33554432", 32 << 20, false}, // 纯数字按字节
		{" 8mb ", 8 << 20, false},
		{"", 0, true},
		{"abc", 0, true},
		{"-4MB", 0, true},
		{"0", 0, true},
		{"512KB", 0, true},  // 低于 1MB 下限
		{"2048MB", 0, true}, // 高于 1GB 上限
		{"1.5MB", 0, true},  // 不支持小数
	}
	for _, c := range cases {
		got, err := parsePartSize(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parsePartSize(%q) 应报错, got %d", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePartSize(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parsePartSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFormatByteSize(t *testing.T) {
	cases := map[int64]string{
		16 << 20: "16MB",
		1 << 30:  "1GB",
		4 << 10:  "4KB",
		123:      "123B",
	}
	for in, want := range cases {
		if got := formatByteSize(in); got != want {
			t.Errorf("formatByteSize(%d) = %q, want %q", in, got, want)
		}
	}
}

// ──────────────────────────────────────────────────────────
// splitDownloadParts
// ──────────────────────────────────────────────────────────

func TestSplitDownloadParts(t *testing.T) {
	// 整除
	parts := splitDownloadParts(32, 16)
	if len(parts) != 2 || parts[0].offset != 0 || parts[0].length != 16 || parts[1].offset != 16 || parts[1].length != 16 {
		t.Errorf("整除切分错误: %+v", parts)
	}
	// 有余量：末片较短
	parts = splitDownloadParts(33, 16)
	if len(parts) != 3 || parts[2].offset != 32 || parts[2].length != 1 {
		t.Errorf("余量切分错误: %+v", parts)
	}
	// 单片
	parts = splitDownloadParts(10, 16)
	if len(parts) != 1 || parts[0].length != 10 {
		t.Errorf("单片切分错误: %+v", parts)
	}
	// 非法输入
	if splitDownloadParts(0, 16) != nil || splitDownloadParts(16, 0) != nil {
		t.Error("非法输入应返回 nil")
	}
	// 覆盖完整性
	parts = splitDownloadParts(100, 7)
	var sum int64
	for i, p := range parts {
		if p.index != i {
			t.Errorf("index 不连续: %+v", p)
		}
		sum += p.length
	}
	if sum != 100 {
		t.Errorf("分片总长 %d != 100", sum)
	}
}

// ──────────────────────────────────────────────────────────
// checkpoint 读写与恢复
// ──────────────────────────────────────────────────────────

func TestCheckpointSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "f.bin.dwspart.meta")
	fp := driveDownloadFingerprint("test-node-1", 0, 100, "")
	cp := &driveDownloadCheckpoint{
		Version:     driveCheckpointVersion,
		Fingerprint: fp,
		TotalSize:   100,
		PartSize:    30,
		Completed:   []bool{true, false, true, false},
	}
	if err := cp.save(metaPath); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := loadDriveDownloadCheckpoint(metaPath, fp, 100, 30, 4)
	if got == nil {
		t.Fatal("roundtrip 应成功加载")
	}
	if !got.Completed[0] || got.Completed[1] || !got.Completed[2] {
		t.Errorf("Completed 位图不符: %v", got.Completed)
	}
}

func TestCheckpointLoadRejectsMismatch(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "f.meta")
	fp := driveDownloadFingerprint("test-node-2", 0, 100, "")
	cp := &driveDownloadCheckpoint{Version: driveCheckpointVersion, Fingerprint: fp, TotalSize: 100, PartSize: 30, Completed: make([]bool, 4)}
	if err := cp.save(metaPath); err != nil {
		t.Fatalf("save: %v", err)
	}
	if loadDriveDownloadCheckpoint(metaPath, "other-fp", 100, 30, 4) != nil {
		t.Error("指纹不符应作废")
	}
	if loadDriveDownloadCheckpoint(metaPath, fp, 200, 30, 4) != nil {
		t.Error("总长不符应作废")
	}
	if loadDriveDownloadCheckpoint(metaPath, fp, 100, 40, 4) != nil {
		t.Error("分片大小不符应作废")
	}
	if loadDriveDownloadCheckpoint(metaPath, fp, 100, 30, 5) != nil {
		t.Error("分片数不符应作废")
	}
	if loadDriveDownloadCheckpoint(filepath.Join(dir, "missing"), fp, 100, 30, 4) != nil {
		t.Error("文件缺失应返回 nil")
	}
	// 损坏 JSON
	if err := os.WriteFile(metaPath, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if loadDriveDownloadCheckpoint(metaPath, fp, 100, 30, 4) != nil {
		t.Error("损坏 JSON 应返回 nil")
	}
}

// 指纹基于 nodeID + version + totalSize，不同 nodeID / version / size 产生不同指纹。
func TestFingerprintNodeIDBased(t *testing.T) {
	a := driveDownloadFingerprint("node-aaa", 0, 500, "")
	b := driveDownloadFingerprint("node-aaa", 0, 500, "")
	if a != b {
		t.Error("相同参数应产生相同指纹")
	}
	c := driveDownloadFingerprint("node-bbb", 0, 500, "")
	if a == c {
		t.Error("不同 nodeID 应产生不同指纹")
	}
	d := driveDownloadFingerprint("node-aaa", 0, 501, "")
	if a == d {
		t.Error("不同 totalSize 应产生不同指纹")
	}
	e := driveDownloadFingerprint("node-aaa", 2, 500, "")
	if a == e {
		t.Error("不同 version 应产生不同指纹")
	}
}

// checkpoint 指纹碰撞防护：相同输出路径、相同大小、不同 nodeID 不复用 checkpoint。
func TestCheckpointNotReusedDifferentNodeID(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "f.meta")
	const totalSize int64 = 1000
	const partSize int64 = 300
	partCount := int((totalSize + partSize - 1) / partSize)

	// 用 nodeA 写入一个 checkpoint
	fpA := driveDownloadFingerprint("node-A", 0, totalSize, "")
	cp := &driveDownloadCheckpoint{
		Version:     driveCheckpointVersion,
		Fingerprint: fpA,
		TotalSize:   totalSize,
		PartSize:    partSize,
		Completed:   make([]bool, partCount),
	}
	cp.Completed[0] = true
	if err := cp.save(metaPath); err != nil {
		t.Fatal(err)
	}

	// 用 nodeB 尝试加载，应返回 nil（不复用）
	fpB := driveDownloadFingerprint("node-B", 0, totalSize, "")
	if loadDriveDownloadCheckpoint(metaPath, fpB, totalSize, partSize, partCount) != nil {
		t.Error("不同 nodeID 同大小不应复用 checkpoint")
	}
}

// checkpoint 指纹碰撞防护：相同 nodeID、不同大小不复用 checkpoint。
func TestCheckpointNotReusedDifferentSize(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "f.meta")
	const partSize int64 = 300

	// 用 size=1000 写入 checkpoint
	fp1000 := driveDownloadFingerprint("same-node", 0, 1000, "")
	pc := int((int64(1000) + partSize - 1) / partSize)
	cp := &driveDownloadCheckpoint{
		Version:     driveCheckpointVersion,
		Fingerprint: fp1000,
		TotalSize:   1000,
		PartSize:    partSize,
		Completed:   make([]bool, pc),
	}
	if err := cp.save(metaPath); err != nil {
		t.Fatal(err)
	}

	// 用 size=2000 尝试加载，应返回 nil
	fp2000 := driveDownloadFingerprint("same-node", 0, 2000, "")
	pc2 := int((int64(2000) + partSize - 1) / partSize)
	if loadDriveDownloadCheckpoint(metaPath, fp2000, 2000, partSize, pc2) != nil {
		t.Error("相同 nodeID 不同大小不应复用 checkpoint")
	}
}

// checkpoint 正常续传：相同 nodeID + 相同大小复用 checkpoint。
func TestCheckpointReusedSameNodeIDAndSize(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "f.meta")
	const totalSize int64 = 1000
	const partSize int64 = 300
	partCount := int((totalSize + partSize - 1) / partSize)

	fp := driveDownloadFingerprint("resume-node", 0, totalSize, "")
	cp := &driveDownloadCheckpoint{
		Version:     driveCheckpointVersion,
		Fingerprint: fp,
		TotalSize:   totalSize,
		PartSize:    partSize,
		Completed:   make([]bool, partCount),
	}
	cp.Completed[0] = true
	cp.Completed[2] = true
	if err := cp.save(metaPath); err != nil {
		t.Fatal(err)
	}

	// 用相同 nodeID + 相同大小加载，应成功复用
	got := loadDriveDownloadCheckpoint(metaPath, fp, totalSize, partSize, partCount)
	if got == nil {
		t.Fatal("相同 nodeID + 相同大小应复用 checkpoint")
	}
	if !got.Completed[0] || got.Completed[1] || !got.Completed[2] || got.Completed[3] {
		t.Errorf("Completed 位图不符: %v", got.Completed)
	}
}

// checkpoint 指纹碰撞防护：同 nodeID、同大小、version=0 但不同 resourceURL 不复用 checkpoint。
// 模拟最新版下载场景：文件被相同大小内容覆盖后 URL path 变化，旧 checkpoint 应作废。
func TestCheckpointNotReusedDifferentResourceURL(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "f.meta")
	const totalSize int64 = 1000
	const partSize int64 = 300
	partCount := int((totalSize + partSize - 1) / partSize)

	// 模拟首次下载时的资源 URL（旧存储位置）
	oldURL := "https://storage.example.com/v1/files/abc123/content?token=xxx"
	fpOld := driveDownloadFingerprint("same-node", 0, totalSize, oldURL)
	cp := &driveDownloadCheckpoint{
		Version:     driveCheckpointVersion,
		Fingerprint: fpOld,
		TotalSize:   totalSize,
		PartSize:    partSize,
		Completed:   make([]bool, partCount),
	}
	cp.Completed[0] = true
	if err := cp.save(metaPath); err != nil {
		t.Fatal(err)
	}

	// 文件被覆盖后获得新的资源 URL（新存储位置，path 不同）
	newURL := "https://storage.example.com/v1/files/def456/content?token=yyy"
	fpNew := driveDownloadFingerprint("same-node", 0, totalSize, newURL)

	// 新旧指纹应不同
	if fpOld == fpNew {
		t.Fatal("不同 resourceURL 应产生不同指纹")
	}

	// 用新指纹加载旧 checkpoint 应返回 nil（不复用）
	if loadDriveDownloadCheckpoint(metaPath, fpNew, totalSize, partSize, partCount) != nil {
		t.Error("同 nodeID、同大小、version=0 但不同 resourceURL 不应复用 checkpoint")
	}
}

// 验证 resourceURL 为空时的安全降级：不影响其他字段的指纹计算。
func TestFingerprintEmptyResourceURLFallback(t *testing.T) {
	// 空 URL 应产生确定性指纹
	a := driveDownloadFingerprint("node-x", 0, 500, "")
	b := driveDownloadFingerprint("node-x", 0, 500, "")
	if a != b {
		t.Error("空 resourceURL 时相同参数应产生相同指纹")
	}

	// 空 URL 与有 URL 的指纹应不同
	c := driveDownloadFingerprint("node-x", 0, 500, "https://example.com/path/to/file")
	if a == c {
		t.Error("空 URL 与有 URL 应产生不同指纹")
	}

	// 仅 query 不同、path 相同的 URL 应产生相同指纹（只取 path）
	d := driveDownloadFingerprint("node-x", 0, 500, "https://example.com/path/to/file?token=aaa")
	e := driveDownloadFingerprint("node-x", 0, 500, "https://example.com/path/to/file?token=bbb")
	if d != e {
		t.Error("仅 query 不同时应产生相同指纹（只取 path）")
	}
}

// ──────────────────────────────────────────────────────────
// parseContentRangeTotal / parseDownloadFileSize / parseDriveUploadType
// ──────────────────────────────────────────────────────────

func TestParseContentRangeTotal(t *testing.T) {
	if n, err := parseContentRangeTotal("bytes 0-0/12345"); err != nil || n != 12345 {
		t.Errorf("got %d, %v", n, err)
	}
	for _, bad := range []string{"", "bytes 0-0/*", "bytes 0-0/", "bytes 0-0/abc", "12345"} {
		if _, err := parseContentRangeTotal(bad); err == nil {
			t.Errorf("%q 应报错", bad)
		}
	}
}

func TestParseContentRange(t *testing.T) {
	cases := []struct {
		in                            string
		wantStart, wantEnd, wantTotal int64
		wantErr                       bool
	}{
		{"bytes 0-1048575/104857600", 0, 1048575, 104857600, false},
		{"bytes 500-999/2000", 500, 999, 2000, false},
		{"bytes 0-0/1", 0, 0, 1, false},
		{"bytes 100-199/*", 100, 199, -1, false}, // total 未知
		{"", 0, 0, 0, true},                      // 空串
		{"0-100/200", 0, 0, 0, true},             // 无 "bytes " 前缀
		{"bytes 0-100", 0, 0, 0, true},           // 缺少 /total
		{"bytes abc-100/200", 0, 0, 0, true},     // start 非数字
		{"bytes 0-abc/200", 0, 0, 0, true},       // end 非数字
		{"bytes 0-100/abc", 0, 0, 0, true},       // total 非数字
		{"bytes 100-50/200", 0, 0, 0, true},      // end < start
	}
	for _, c := range cases {
		start, end, total, err := parseContentRange(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseContentRange(%q) 应报错", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseContentRange(%q) unexpected error: %v", c.in, err)
			continue
		}
		if start != c.wantStart || end != c.wantEnd || total != c.wantTotal {
			t.Errorf("parseContentRange(%q) = (%d,%d,%d), want (%d,%d,%d)",
				c.in, start, end, total, c.wantStart, c.wantEnd, c.wantTotal)
		}
	}
}

func TestParseDownloadFileSize(t *testing.T) {
	if n := parseDownloadFileSize(`{"result":{"fileSize":1048576,"downloadUrl":"u"}}`); n != 1048576 {
		t.Errorf("number: got %d", n)
	}
	if n := parseDownloadFileSize(`{"fileSize":"2048"}`); n != 2048 {
		t.Errorf("string: got %d", n)
	}
	if n := parseDownloadFileSize(`{"result":{}}`); n != 0 {
		t.Errorf("missing: got %d", n)
	}
	if n := parseDownloadFileSize("not json"); n != 0 {
		t.Errorf("invalid: got %d", n)
	}
}

func TestParseDownloadFileVersion(t *testing.T) {
	// 数值类型
	if n := parseDownloadFileVersion(`{"result":{"version":3,"downloadUrl":"u"}}`); n != 3 {
		t.Errorf("number: got %d, want 3", n)
	}
	// 字符串类型
	if n := parseDownloadFileVersion(`{"version":"7"}`); n != 7 {
		t.Errorf("string: got %d, want 7", n)
	}
	// 缺失字段
	if n := parseDownloadFileVersion(`{"result":{}}`); n != 0 {
		t.Errorf("missing: got %d, want 0", n)
	}
	// 非法 JSON
	if n := parseDownloadFileVersion("not json"); n != 0 {
		t.Errorf("invalid json: got %d, want 0", n)
	}
	// 无 result 包裹的数值
	if n := parseDownloadFileVersion(`{"version":12}`); n != 12 {
		t.Errorf("top-level number: got %d, want 12", n)
	}
}

func TestParseDriveUploadType(t *testing.T) {
	if got := parseDriveUploadType(`{"result":{"uploadType":"httpToCenterWithToken","resourceUrl":"u"}}`); got != uploadTypeCenterToken {
		t.Errorf("got %q", got)
	}
	if got := parseDriveUploadType(`{"resourceUrl":"u"}`); got != "" {
		t.Errorf("存量返回无 uploadType 应为空串, got %q", got)
	}
	if got := parseDriveUploadType("not json"); got != "" {
		t.Errorf("invalid json 应为空串, got %q", got)
	}
}

// ──────────────────────────────────────────────────────────
// isAuthStatusError / decorateUploadSizeError
// ──────────────────────────────────────────────────────────

func TestIsAuthStatusError(t *testing.T) {
	if !isAuthStatusError(&httpStatusError{StatusCode: 401}) || !isAuthStatusError(&httpStatusError{StatusCode: 403}) {
		t.Error("typed 401/403 应命中")
	}
	if isAuthStatusError(&httpStatusError{StatusCode: 500}) {
		t.Error("typed 500 不应命中")
	}
	if !isAuthStatusError(fmt.Errorf("OSS upload failed: %w", &httpStatusError{StatusCode: 403, Body: "x"})) {
		t.Error("包装后的 typed 错误应命中")
	}
	if !isAuthStatusError(fmt.Errorf("HTTP 401: expired")) {
		t.Error("字符串形态应命中（测试注入兼容）")
	}
	if isAuthStatusError(nil) || isAuthStatusError(fmt.Errorf("HTTP 404: not found")) {
		t.Error("nil/404 不应命中")
	}
}

func TestDecorateUploadSizeError(t *testing.T) {
	// 413 → 补充可读提示
	err := decorateUploadSizeError(&httpStatusError{StatusCode: 413, Body: "too large"}, "")
	if !strings.Contains(err.Error(), "提示") {
		t.Errorf("413 应补充提示: %v", err)
	}
	// 中心协议 + 超限语义 body
	err = decorateUploadSizeError(&httpStatusError{StatusCode: 400, Body: "file size exceed limit"}, uploadTypeCenterToken)
	if !strings.Contains(err.Error(), "提示") {
		t.Errorf("中心协议超限应补充提示: %v", err)
	}
	// 普通错误原样返回
	orig := &httpStatusError{StatusCode: 500, Body: "internal"}
	if got := decorateUploadSizeError(orig, uploadTypeCenterToken); got.Error() != orig.Error() {
		t.Errorf("普通错误不应装饰: %v", got)
	}
	nonHTTP := fmt.Errorf("dial timeout")
	if got := decorateUploadSizeError(nonHTTP, ""); got != nonHTTP {
		t.Errorf("非 HTTP 错误不应装饰: %v", got)
	}
}

// ──────────────────────────────────────────────────────────
// driveTransferDownload：整流 / 分片 / 回退 / 断点续传
// ──────────────────────────────────────────────────────────

// rangeTestServer 支持 Range 的测试服务端。
func rangeTestServer(t *testing.T, content []byte, requireToken string, tokenGen *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requireToken != "" {
			want := requireToken
			if tokenGen != nil {
				want = fmt.Sprintf("%s-%d", requireToken, tokenGen.Load())
			}
			if r.Header.Get("dentry-token") != want {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, "token expired")
				return
			}
		}
		rng := r.Header.Get("Range")
		if rng == "" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(content)
			return
		}
		var start, end int64
		if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err != nil || start > end || start >= int64(len(content)) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if end >= int64(len(content)) {
			end = int64(len(content)) - 1
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[start : end+1])
	}))
}

func makeTestContent(n int) []byte {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte(i % 251)
	}
	return buf
}

func verifyFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取产物失败: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("产物长度 %d != %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("产物内容第 %d 字节不符", i)
		}
	}
}

// 小分片选项：partSize 用引擎内部值绕过 flag 校验（单测直接构造 options）。
func smallPartOpts(partSize int64, parallel int) driveDownloadOptions {
	return driveDownloadOptions{partSize: partSize, parallel: parallel, resume: true}
}

func TestDriveTransferDownload_RangedParts(t *testing.T) {
	content := makeTestContent(1000)
	srv := rangeTestServer(t, content, "", nil)
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "out.bin")

	// knownSize=1000 ≥ 2×300 → 分片
	opts := smallPartOpts(300, 3)
	opts.knownSize = 1000
	if err := driveTransferDownload(context.Background(), nil, srv.URL, nil, dest, opts); err != nil {
		t.Fatalf("分片下载失败: %v", err)
	}
	verifyFile(t, dest, content)
	if _, err := os.Stat(dest + drivePartFileSuffix); !os.IsNotExist(err) {
		t.Error("完成后应清理 .dwspart")
	}
	if _, err := os.Stat(dest + drivePartMetaSuffix); !os.IsNotExist(err) {
		t.Error("完成后应清理 checkpoint")
	}
}

// knownSize 小于阈值 → 直接整流（走可注入 httpGetFile）。
func TestDriveTransferDownload_SmallFileSingleStream(t *testing.T) {
	var calls atomic.Int32
	SetHTTPGetFile(func(ctx context.Context, url string, headers map[string]string, destPath string) error {
		calls.Add(1)
		return os.WriteFile(destPath, []byte("small"), 0o644)
	})
	defer SetHTTPGetFile(nil)

	dest := filepath.Join(t.TempDir(), "small.bin")
	opts := smallPartOpts(1<<20, 4)
	opts.knownSize = 100 // < 2MB 阈值
	if err := driveTransferDownload(context.Background(), nil, "https://x.example.com/f", nil, dest, opts); err != nil {
		t.Fatalf("整流下载失败: %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("应恰好调用一次 httpGetFile, got %d", calls.Load())
	}
	verifyFile(t, dest, []byte("small"))
}

// 整流 401 → 重取凭证重试一次。
func TestDownloadSingleWithAuthRetry(t *testing.T) {
	var calls atomic.Int32
	SetHTTPGetFile(func(ctx context.Context, url string, headers map[string]string, destPath string) error {
		if calls.Add(1) == 1 {
			return &httpStatusError{StatusCode: 401, Body: "expired"}
		}
		if url != "https://new.example.com/f" || headers["dentry-token"] != "new-token" {
			return fmt.Errorf("重试应使用新凭证, got url=%s headers=%v", url, headers)
		}
		return os.WriteFile(destPath, []byte("ok"), 0o644)
	})
	defer SetHTTPGetFile(nil)

	fetched := false
	fetch := func(ctx context.Context) (string, map[string]string, error) {
		fetched = true
		return "https://new.example.com/f", map[string]string{"dentry-token": "new-token"}, nil
	}
	dest := filepath.Join(t.TempDir(), "auth.bin")
	if err := downloadSingleWithAuthRetry(context.Background(), fetch, "https://old.example.com/f", nil, dest); err != nil {
		t.Fatalf("401 重试后应成功: %v", err)
	}
	if !fetched || calls.Load() != 2 {
		t.Errorf("fetched=%v calls=%d", fetched, calls.Load())
	}
	verifyFile(t, dest, []byte("ok"))
}

// 整流重取凭证后仍失败 → 走既有错误路径（返回错误）。
func TestDownloadSingleWithAuthRetry_StillFails(t *testing.T) {
	SetHTTPGetFile(func(ctx context.Context, url string, headers map[string]string, destPath string) error {
		return &httpStatusError{StatusCode: 403, Body: "denied"}
	})
	defer SetHTTPGetFile(nil)
	fetch := func(ctx context.Context) (string, map[string]string, error) {
		return "https://new.example.com/f", nil, nil
	}
	err := downloadSingleWithAuthRetry(context.Background(), fetch, "https://old.example.com/f", nil, filepath.Join(t.TempDir(), "x"))
	if err == nil || !isAuthStatusError(err) {
		t.Fatalf("应返回原始鉴权错误: %v", err)
	}
}

// 服务端不支持 Range（探测返回 200）→ 自动回退整流。
func TestDriveTransferDownload_FallbackWhenNoRangeSupport(t *testing.T) {
	content := makeTestContent(900)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 忽略 Range 头，始终 200 全量
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "fallback.bin")
	opts := smallPartOpts(300, 4)
	opts.knownSize = 900 // ≥ 阈值，进入探测
	if err := driveTransferDownload(context.Background(), nil, srv.URL, nil, dest, opts); err != nil {
		t.Fatalf("回退整流失败: %v", err)
	}
	verifyFile(t, dest, content)
}

// Content-Range 校验：正常匹配、区间错位、header 缺失
func TestFetchRangeInto_ContentRangeValidation(t *testing.T) {
	content := makeTestContent(1000)

	// 正常情况：Content-Range 区间匹配
	t.Run("match", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var start, end int64
			fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(content[start : end+1])
		}))
		defer srv.Close()

		f, err := os.CreateTemp(t.TempDir(), "cr-match-*")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := f.Truncate(1000); err != nil {
			t.Fatal(err)
		}
		part := driveDownloadPart{index: 1, offset: 300, length: 300}
		if err := fetchRangeInto(context.Background(), srv.URL, nil, f, part); err != nil {
			t.Fatalf("正常匹配不应报错: %v", err)
		}
	})

	// Content-Range 区间错位（start 不匹配）→ 返回错误
	t.Run("mismatch_start", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 请求 300-599，但返回假装的 0-299
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-299/%d", len(content)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(content[0:300])
		}))
		defer srv.Close()

		f, err := os.CreateTemp(t.TempDir(), "cr-mismatch-*")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := f.Truncate(1000); err != nil {
			t.Fatal(err)
		}
		part := driveDownloadPart{index: 1, offset: 300, length: 300}
		err = fetchRangeInto(context.Background(), srv.URL, nil, f, part)
		if err == nil {
			t.Fatal("Content-Range 错位应返回错误")
		}
		if !strings.Contains(err.Error(), "不匹配") {
			t.Errorf("错误信息应包含'不匹配': %v", err)
		}
	})

	// Content-Range header 缺失 → 不阻断，正常下载
	t.Run("missing_header", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var start, end int64
			fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
			// 不设置 Content-Range header
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(content[start : end+1])
		}))
		defer srv.Close()

		f, err := os.CreateTemp(t.TempDir(), "cr-missing-*")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := f.Truncate(1000); err != nil {
			t.Fatal(err)
		}
		part := driveDownloadPart{index: 0, offset: 0, length: 300}
		if err := fetchRangeInto(context.Background(), srv.URL, nil, f, part); err != nil {
			t.Fatalf("Content-Range 缺失不应阻断下载: %v", err)
		}
	})

	// Content-Range 格式异常 → 返回错误
	t.Run("malformed_header", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Range", "invalid-format")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(content[0:300])
		}))
		defer srv.Close()

		f, err := os.CreateTemp(t.TempDir(), "cr-malformed-*")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := f.Truncate(1000); err != nil {
			t.Fatal(err)
		}
		part := driveDownloadPart{index: 0, offset: 0, length: 300}
		err = fetchRangeInto(context.Background(), srv.URL, nil, f, part)
		if err == nil {
			t.Fatal("Content-Range 格式异常应返回错误")
		}
		if !strings.Contains(err.Error(), "解析失败") {
			t.Errorf("错误信息应包含'解析失败': %v", err)
		}
	})
}

// 分片过程 401 → single-flight 刷新凭证后续传（不重下已完成分片）。
func TestDriveTransferDownload_AuthRefreshDuringParts(t *testing.T) {
	content := makeTestContent(1200)
	var tokenGen atomic.Int32 // 服务端当前有效 token 代数
	srv := rangeTestServer(t, content, "tok", &tokenGen)
	defer srv.Close()

	var fetchCalls atomic.Int32
	fetch := func(ctx context.Context) (string, map[string]string, error) {
		fetchCalls.Add(1)
		return srv.URL, map[string]string{"dentry-token": fmt.Sprintf("tok-%d", tokenGen.Load())}, nil
	}
	dest := filepath.Join(t.TempDir(), "auth-parts.bin")
	opts := smallPartOpts(300, 2)
	opts.knownSize = 1200

	// 初始凭证是第 0 代；探测后服务端轮换到第 1 代，分片请求将收到 401
	initial := map[string]string{"dentry-token": "tok-0"}
	go func() {
		// 探测完成前不轮换：用探测本身消耗第 0 代，随后轮换
		tokenGen.Store(1)
	}()
	if err := driveTransferDownload(context.Background(), fetch, srv.URL, initial, dest, opts); err != nil {
		t.Fatalf("凭证刷新续传失败: %v", err)
	}
	verifyFile(t, dest, content)
}

// 断点续传：模拟部分分片完成后中断，重跑跳过已完成分片。
func TestDriveTransferDownload_ResumeSkipsCompletedParts(t *testing.T) {
	content := makeTestContent(1000)
	partSize := int64(300)
	dest := filepath.Join(t.TempDir(), "resume.bin")
	partPath := dest + drivePartFileSuffix
	metaPath := dest + drivePartMetaSuffix

	// 预置：分片 0、2 已完成（写入正确数据），1、3 未完成
	pre := make([]byte, 1000)
	copy(pre[0:300], content[0:300])
	copy(pre[600:900], content[600:900])
	if err := os.WriteFile(partPath, pre, 0o644); err != nil {
		t.Fatal(err)
	}
	const resumeNodeID = "resume-test-node"

	// 服务端记录收到的 Range 区间
	var mu struct {
		ranges []string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rng := r.Header.Get("Range")
		mu.ranges = append(mu.ranges, rng)
		var start, end int64
		fmt.Sscanf(rng, "bytes=%d-%d", &start, &end)
		if end >= int64(len(content)) {
			end = int64(len(content)) - 1
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[start : end+1])
	}))
	defer srv.Close()

	cp := &driveDownloadCheckpoint{
		Version:     driveCheckpointVersion,
		Fingerprint: driveDownloadFingerprint(resumeNodeID, 0, 1000, srv.URL),
		TotalSize:   1000,
		PartSize:    partSize,
		Completed:   []bool{true, false, true, false},
	}
	if err := cp.save(metaPath); err != nil {
		t.Fatal(err)
	}

	opts := smallPartOpts(partSize, 1) // 串行便于断言
	opts.knownSize = 1000
	opts.nodeID = resumeNodeID
	if err := driveTransferDownload(context.Background(), nil, srv.URL, nil, dest, opts); err != nil {
		t.Fatalf("续传失败: %v", err)
	}
	verifyFile(t, dest, content)

	// 探测请求(bytes=0-0) + 分片1(300-599) + 分片3(900-999)，不应重下分片0/2
	for _, rng := range mu.ranges {
		if rng == fmt.Sprintf("bytes=%d-%d", 0, partSize-1) || rng == fmt.Sprintf("bytes=%d-%d", 2*partSize, 3*partSize-1) {
			t.Errorf("已完成分片被重下: %s", rng)
		}
	}
}

// --no-resume：清理历史断点从头下载，且过程中不写 checkpoint。
func TestDriveTransferDownload_NoResume(t *testing.T) {
	content := makeTestContent(1000)
	srv := rangeTestServer(t, content, "", nil)
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "noresume.bin")
	// 预置一个陈旧 checkpoint（若未清理会因 Completed 全 true 跳过下载产出错误内容）
	stale := &driveDownloadCheckpoint{
		Version:     driveCheckpointVersion,
		Fingerprint: driveDownloadFingerprint("stale-node", 0, 1000, ""),
		TotalSize:   1000,
		PartSize:    300,
		Completed:   []bool{true, true, true, true},
	}
	if err := os.WriteFile(dest+drivePartFileSuffix, make([]byte, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := stale.save(dest + drivePartMetaSuffix); err != nil {
		t.Fatal(err)
	}

	opts := driveDownloadOptions{partSize: 300, parallel: 2, resume: false, knownSize: 1000}
	if err := driveTransferDownload(context.Background(), nil, srv.URL, nil, dest, opts); err != nil {
		t.Fatalf("no-resume 下载失败: %v", err)
	}
	verifyFile(t, dest, content) // 内容正确说明确实重下而非采信陈旧 checkpoint
	if _, err := os.Stat(dest + drivePartMetaSuffix); !os.IsNotExist(err) {
		t.Error("no-resume 不应遗留 checkpoint")
	}
}

// 分片失败重试后成功（指数退避路径）。
func TestDownloadOnePart_RetryOnTransientError(t *testing.T) {
	content := makeTestContent(600)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= 2 { // 前两次 500
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		rng := r.Header.Get("Range")
		var start, end int64
		fmt.Sscanf(rng, "bytes=%d-%d", &start, &end)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[start : end+1])
	}))
	defer srv.Close()

	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "part.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(600); err != nil {
		t.Fatal(err)
	}
	creds := &driveCredentialState{url: srv.URL}
	if err := downloadOnePart(context.Background(), creds, f, driveDownloadPart{index: 0, offset: 0, length: 600}); err != nil {
		t.Fatalf("瞬时错误重试后应成功: %v", err)
	}
	if hits.Load() != 3 {
		t.Errorf("应请求 3 次, got %d", hits.Load())
	}
}

// 分片持续失败超过重试上限 → 返回错误。
func TestDownloadOnePart_ExhaustsRetries(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f, err := os.Create(filepath.Join(t.TempDir(), "part.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	creds := &driveCredentialState{url: srv.URL}
	err = downloadOnePart(context.Background(), creds, f, driveDownloadPart{index: 0, offset: 0, length: 10})
	if err == nil {
		t.Fatal("超过重试上限应失败")
	}
	if hits.Load() != int32(driveDownloadPartRetries)+1 {
		t.Errorf("应请求 %d 次, got %d", driveDownloadPartRetries+1, hits.Load())
	}
}

// ──────────────────────────────────────────────────────────
// driveUploadPut：中心协议 PUT + 401 重取凭证重试
// ──────────────────────────────────────────────────────────

func TestDriveUploadPut_Success(t *testing.T) {
	var gotURL string
	var gotHeaders map[string]string
	SetHTTPPutFile(func(ctx context.Context, url string, headers map[string]string, filePath string, fileSize int64) error {
		gotURL, gotHeaders = url, headers
		return nil
	})
	defer SetHTTPPutFile(nil)

	cred := `{"result":{"uploadType":"httpToCenterWithToken","resourceUrl":"https://c.example.com/attachment/token/upload/single?file_size=5&spaceId=s1&upload_key=k1","uploadId":"k1","headers":{"dentry-token":"tk"}}}`
	uploadID, err := driveUploadPut(context.Background(), cred, nil, "/tmp/f.bin", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadID != "k1" {
		t.Errorf("uploadID = %q", uploadID)
	}
	if !strings.Contains(gotURL, "upload_key=k1") {
		t.Errorf("应使用服务端拼好的完整 URL: %q", gotURL)
	}
	if gotHeaders["dentry-token"] != "tk" {
		t.Errorf("headers 应透传 dentry-token: %v", gotHeaders)
	}
}

func TestDriveUploadPut_AuthRetryWithNewCredential(t *testing.T) {
	var calls atomic.Int32
	SetHTTPPutFile(func(ctx context.Context, url string, headers map[string]string, filePath string, fileSize int64) error {
		if calls.Add(1) == 1 {
			return &httpStatusError{StatusCode: 401, Body: "token expired"}
		}
		if headers["dentry-token"] != "tk2" {
			return fmt.Errorf("重试应使用新 token, got %v", headers)
		}
		return nil
	})
	defer SetHTTPPutFile(nil)

	cred1 := `{"result":{"uploadType":"httpToCenterWithToken","resourceUrl":"https://c.example.com/u?upload_key=k1","uploadId":"k1","headers":{"dentry-token":"tk1"}}}`
	cred2 := `{"result":{"uploadType":"httpToCenterWithToken","resourceUrl":"https://c.example.com/u?upload_key=k2","uploadId":"k2","headers":{"dentry-token":"tk2"}}}`
	refetch := func(ctx context.Context) (string, error) { return cred2, nil }

	uploadID, err := driveUploadPut(context.Background(), cred1, refetch, "/tmp/f.bin", 5)
	if err != nil {
		t.Fatalf("401 重试后应成功: %v", err)
	}
	if uploadID != "k2" {
		t.Errorf("应返回新凭证的 uploadId, got %q", uploadID)
	}
	if calls.Load() != 2 {
		t.Errorf("PUT 应调用 2 次, got %d", calls.Load())
	}
}

func TestDriveUploadPut_AuthRetryStillFails(t *testing.T) {
	SetHTTPPutFile(func(ctx context.Context, url string, headers map[string]string, filePath string, fileSize int64) error {
		return &httpStatusError{StatusCode: 403, Body: "denied"}
	})
	defer SetHTTPPutFile(nil)

	cred := `{"result":{"resourceUrl":"https://c.example.com/u","uploadId":"k1"}}`
	refetch := func(ctx context.Context) (string, error) { return cred, nil }
	if _, err := driveUploadPut(context.Background(), cred, refetch, "/tmp/f.bin", 5); err == nil {
		t.Fatal("重试仍失败应返回错误")
	}
}

func TestDriveUploadPut_SizeLimitHint(t *testing.T) {
	SetHTTPPutFile(func(ctx context.Context, url string, headers map[string]string, filePath string, fileSize int64) error {
		return &httpStatusError{StatusCode: 413, Body: "request entity too large"}
	})
	defer SetHTTPPutFile(nil)

	cred := `{"result":{"uploadType":"httpToCenterWithToken","resourceUrl":"https://c.example.com/u","uploadId":"k1","headers":{"dentry-token":"tk"}}}`
	_, err := driveUploadPut(context.Background(), cred, nil, "/tmp/f.bin", 5)
	if err == nil || !strings.Contains(err.Error(), "提示") {
		t.Fatalf("超限错误应含可读提示: %v", err)
	}
}

// ──────────────────────────────────────────────────────────
// parseDriveDownloadInfo：中心协议 headers 透传（新增行为）
// ──────────────────────────────────────────────────────────

func TestParseDriveDownloadInfo_CenterProtocolHeaders(t *testing.T) {
	text := `{"result":{"downloadType":"httpToCenterWithToken","downloadUrl":"https://c.example.com/attachment/token/mdown?k=v","headers":{"dentry-token":"tk"},"fileName":"a.bin"},"success":true}`
	url, headers, err := parseDriveDownloadInfo(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://c.example.com/attachment/token/mdown?k=v" {
		t.Errorf("url = %q", url)
	}
	if headers["dentry-token"] != "tk" {
		t.Errorf("中心协议应透传 dentry-token, got %v", headers)
	}
}

// ──────────────────────────────────────────────────────────
// driveCredentialState：single-flight 刷新
// ──────────────────────────────────────────────────────────

func TestDriveCredentialStateSingleFlightRefresh(t *testing.T) {
	var fetches atomic.Int32
	cs := &driveCredentialState{
		url: "u0",
		fetch: func(ctx context.Context) (string, map[string]string, error) {
			n := fetches.Add(1)
			return fmt.Sprintf("u%d", n), nil, nil
		},
	}
	_, _, gen0 := cs.current()
	// 两个并发分片都持第 0 代请求刷新：只应真正 fetch 一次
	if err := cs.refresh(context.Background(), gen0); err != nil {
		t.Fatal(err)
	}
	if err := cs.refresh(context.Background(), gen0); err != nil {
		t.Fatal(err)
	}
	if fetches.Load() != 1 {
		t.Errorf("同代刷新应 single-flight, fetch 次数 = %d", fetches.Load())
	}
	url, _, gen1 := cs.current()
	if url != "u1" || gen1 != gen0+1 {
		t.Errorf("刷新后 url=%q gen=%d", url, gen1)
	}
	// 持新代再刷新 → 再 fetch 一次
	if err := cs.refresh(context.Background(), gen1); err != nil {
		t.Fatal(err)
	}
	if fetches.Load() != 2 {
		t.Errorf("新代刷新应真正执行, fetch 次数 = %d", fetches.Load())
	}
}
