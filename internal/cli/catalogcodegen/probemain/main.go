// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

// Command probemain exists only to measure the linked size contribution of the
// generated catalog literals; it keeps the data reachable so the linker cannot
// strip it.
package main

import (
	"fmt"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/catalogcodegen"
)

func main() {
	total := 0
	for _, tool := range catalogcodegen.Tools {
		total += len(tool.CLIPath) + len(tool.Description) + len(tool.Parameters)
	}
	fmt.Println(len(catalogcodegen.Tools), total)
}
