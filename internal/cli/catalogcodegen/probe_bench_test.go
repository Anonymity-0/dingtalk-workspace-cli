// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package catalogcodegen_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/catalogcodegen"
)

// BenchmarkGoLiteralAccess measures reaching a tool in the compiled-in map. The
// data is static, so this is a map lookup with no decode and no allocation —
// the cost model internal/shortcut already gets.
func BenchmarkGoLiteralAccess(b *testing.B) {
	if len(catalogcodegen.Tools) == 0 {
		b.Fatal("no generated tools")
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tool, ok := catalogcodegen.Tools["dev.delete_dev_app"]
		if !ok || tool.CLIPath == "" {
			b.Fatal("lookup missed")
		}
	}
}

// BenchmarkJSONDecodeSameShard measures decoding the same product's committed
// shard into untyped maps, i.e. what the current loader pays for this product.
func BenchmarkJSONDecodeSameShard(b *testing.B) {
	data, err := os.ReadFile("../schema_catalog/tools/dev.json")
	if err != nil {
		b.Skip("shard unavailable")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var shard struct {
			Tools map[string]map[string]any `json:"tools"`
		}
		if err := json.Unmarshal(data, &shard); err != nil {
			b.Fatal(err)
		}
		if len(shard.Tools) == 0 {
			b.Fatal("empty shard")
		}
	}
}
