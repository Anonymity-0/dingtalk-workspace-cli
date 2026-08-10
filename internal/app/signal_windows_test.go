//go:build windows

package app

import (
	"syscall"
	"testing"
)

func signalSelf(t *testing.T, _ syscall.Signal) {
	t.Helper()
	t.Skip("Windows does not expose Unix signal delivery to the current process")
}
