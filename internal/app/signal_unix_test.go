//go:build !windows

package app

import (
	"os"
	"syscall"
	"testing"
)

func signalSelf(t *testing.T, sig syscall.Signal) {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), sig); err != nil {
		t.Fatalf("signal process: %v", err)
	}
}
