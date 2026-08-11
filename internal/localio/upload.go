// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package localio

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const defaultUploadLimit = int64(5 << 30)

// UploadResult records only non-sensitive transfer facts. The signed URL is
// deliberately never returned.
type UploadResult struct {
	SizeBytes int64
	Attempts  int
}

// PutFile uploads a regular local file to an exact trusted pre-signed HTTPS
// endpoint. Redirects are rejected so file bytes and credentials cannot move to
// a second origin. Transient failures are retried with the file reopened.
func PutFile(ctx context.Context, rawURL, path string, maxBytes int64) (UploadResult, error) {
	client := secureHTTPClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return putFileWithClient(ctx, rawURL, path, maxBytes, client, func(raw string) error {
		_, err := ValidateDownloadURL(raw)
		return err
	})
}

func putFileWithClient(ctx context.Context, rawURL, path string, maxBytes int64, client *http.Client, validate func(string) error) (UploadResult, error) {
	if validate == nil {
		return UploadResult{}, fmt.Errorf("upload URL validator is nil")
	}
	if err := validate(rawURL); err != nil {
		return UploadResult{}, err
	}
	if maxBytes <= 0 {
		maxBytes = defaultUploadLimit
	}
	info, err := os.Stat(path)
	if err != nil {
		return UploadResult{}, fmt.Errorf("读取上传文件失败: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBytes {
		return UploadResult{}, fmt.Errorf("上传文件必须是 1..%d 字节的普通文件", maxBytes)
	}
	result := UploadResult{SizeBytes: info.Size()}
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		result.Attempts = attempt
		file, err := os.Open(path)
		if err != nil {
			return UploadResult{}, fmt.Errorf("打开上传文件失败: %w", err)
		}
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPut, rawURL, file)
		if requestErr != nil {
			_ = file.Close()
			return UploadResult{}, fmt.Errorf("创建上传请求失败: %w", requestErr)
		}
		request.ContentLength = info.Size()
		response, callErr := client.Do(request)
		_ = file.Close()
		if callErr == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
			bodyCloseErr := response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 && bodyCloseErr == nil {
				return result, nil
			}
			if response.StatusCode < 500 && response.StatusCode != http.StatusRequestTimeout && response.StatusCode != http.StatusTooManyRequests {
				return result, fmt.Errorf("上传返回 HTTP %d", response.StatusCode)
			}
			lastErr = fmt.Errorf("上传返回 HTTP %d", response.StatusCode)
		} else {
			lastErr = callErr
		}
		if attempt < 3 {
			timer := time.NewTimer(time.Duration(attempt) * 200 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return result, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return result, fmt.Errorf("上传在 %d 次尝试后失败: %w", result.Attempts, lastErr)
}
