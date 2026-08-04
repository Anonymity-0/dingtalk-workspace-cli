// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build windows

package chat

import "golang.org/x/sys/windows"

func replaceFileAtomically(source, target string) error {
	return windows.Rename(source, target)
}
