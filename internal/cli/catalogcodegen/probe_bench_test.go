// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package catalogcodegen_test

import (
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
