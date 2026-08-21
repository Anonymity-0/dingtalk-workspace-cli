//go:build !windows

package scripts_test

import (
	"os/exec"
	"syscall"
)

func configureIsolatedProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func processGroupID(pid int) (int, error) {
	return syscall.Getpgid(pid)
}

func signalProcessGroup(groupID int, signal syscall.Signal) error {
	return syscall.Kill(-groupID, signal)
}
