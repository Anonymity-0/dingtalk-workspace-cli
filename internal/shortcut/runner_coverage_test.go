// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package shortcut

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCrossPlatformCoverageRuntimeContextForTest(t *testing.T) {
	cmd := &cobra.Command{Use: "run"}
	rt := RuntimeContextForTest(cmd, Shortcut{Service: "sample", Command: "run"})
	if rt == nil || rt.cmd != cmd || rt.shortcut.Service != "sample" {
		t.Fatalf("RuntimeContextForTest = %#v", rt)
	}
}
