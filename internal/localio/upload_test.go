// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package localio

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCrossPlatformCoveragePutFileRetriesAndUploadsExactBytesE2E(t *testing.T) {
	payload := []byte("minutes-e2e")
	path := filepath.Join(t.TempDir(), "audio.wav")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPut {
			t.Errorf("method = %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != string(payload) {
			t.Errorf("body = %q", body)
		}
		if calls == 1 {
			http.Error(w, "retry", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	result, err := putFileWithClient(context.Background(), server.URL, path, 100, server.Client(), func(string) error { return nil })
	if err != nil || calls != 2 || result.Attempts != 2 || result.SizeBytes != int64(len(payload)) {
		t.Fatalf("upload = %#v calls=%d err=%v", result, calls, err)
	}
}

func TestCrossPlatformCoveragePutFileRejectsRedirectAndInvalidFileE2E(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audio.wav")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer redirect.Close()
	client := redirect.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if _, err := putFileWithClient(context.Background(), redirect.URL, path, 100, client, func(string) error { return nil }); err == nil {
		t.Fatal("redirect accepted")
	}
	if _, err := putFileWithClient(context.Background(), redirect.URL, filepath.Join(t.TempDir(), "missing"), 100, client, func(string) error { return nil }); err == nil {
		t.Fatal("missing file accepted")
	}
}
