// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package localio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCrossPlatformCoveragePublishBytesAtomicNoClobberE2E(t *testing.T) {
	base := t.TempDir()
	result, err := PublishBytes([]byte("verified artifact"), PublishBytesOptions{BaseDir: base, Output: "out/", PreferredName: "transcript.json"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(base, result.RelativePath))
	if err != nil || string(raw) != "verified artifact" || result.SizeBytes != int64(len(raw)) {
		t.Fatalf("publish result=%#v raw=%q err=%v", result, raw, err)
	}
	if _, err := PublishBytes([]byte("overwrite"), PublishBytesOptions{BaseDir: base, Output: "out/transcript.json"}); err == nil {
		t.Fatal("existing output was overwritten")
	}
	if _, err := PublishBytes([]byte("escape"), PublishBytesOptions{BaseDir: base, Output: "../escape"}); err == nil {
		t.Fatal("path escape accepted")
	}
}
