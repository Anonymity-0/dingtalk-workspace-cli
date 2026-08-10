//go:build windows

package app

import (
	"os"
	"syscall"
)

func redeliverProcessSignal(sig os.Signal) {
	if sig == syscall.SIGTERM {
		os.Exit(143)
	}
	os.Exit(130)
}
