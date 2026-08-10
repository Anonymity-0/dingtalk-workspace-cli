//go:build !windows

package app

import (
	"os"
	"syscall"
)

func redeliverProcessSignal(sig os.Signal) {
	if syscallSig, ok := sig.(syscall.Signal); ok {
		_ = syscall.Kill(os.Getpid(), syscallSig)
	}
}
