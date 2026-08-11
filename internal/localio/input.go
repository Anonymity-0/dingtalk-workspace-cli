// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package localio

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const defaultTextInputLimit = int64(8 << 20)

// ReadTextInput resolves a literal, "-" stdin, or @workspace-relative file.
// File symlinks must resolve inside the current working directory and all input
// forms are bounded to prevent accidental unbounded memory use.
func ReadTextInput(spec string, stdin io.Reader, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = defaultTextInputLimit
	}
	if spec == "-" {
		if stdin == nil {
			return "", fmt.Errorf("LOCAL_INPUT_INVALID: stdin 不可用")
		}
		data, err := io.ReadAll(io.LimitReader(stdin, maxBytes+1))
		if err != nil {
			return "", fmt.Errorf("LOCAL_INPUT_READ_FAILED: %w", err)
		}
		if int64(len(data)) > maxBytes {
			return "", fmt.Errorf("LOCAL_INPUT_TOO_LARGE: stdin 超过 %d 字节", maxBytes)
		}
		return string(data), nil
	}
	if !strings.HasPrefix(spec, "@") {
		return spec, nil
	}
	path := strings.TrimSpace(strings.TrimPrefix(spec, "@"))
	if path == "" {
		return "", fmt.Errorf("LOCAL_INPUT_INVALID: @file 路径不能为空")
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("LOCAL_INPUT_UNSAFE: @file 只接受工作目录内的相对路径")
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("LOCAL_INPUT_UNSAFE: @file 不能逃逸工作目录")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("LOCAL_INPUT_READ_FAILED: %w", err)
	}
	realBase, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", fmt.Errorf("LOCAL_INPUT_READ_FAILED: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(filepath.Join(realBase, clean))
	if err != nil {
		return "", fmt.Errorf("LOCAL_INPUT_READ_FAILED: %w", err)
	}
	rel, err := filepath.Rel(realBase, realPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("LOCAL_INPUT_UNSAFE: @file 解析后逃逸工作目录")
	}
	info, err := os.Stat(realPath)
	if err != nil {
		return "", fmt.Errorf("LOCAL_INPUT_READ_FAILED: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("LOCAL_INPUT_INVALID: @file 必须是普通文件")
	}
	if info.Size() > maxBytes {
		return "", fmt.Errorf("LOCAL_INPUT_TOO_LARGE: 文件大小 %d 超过 %d 字节", info.Size(), maxBytes)
	}
	data, err := os.ReadFile(realPath)
	if err != nil {
		return "", fmt.Errorf("LOCAL_INPUT_READ_FAILED: %w", err)
	}
	return string(data), nil
}
