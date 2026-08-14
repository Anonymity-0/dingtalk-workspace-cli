//go:build !windows

package upgrade

import (
	"os"
	"syscall"
)

func testCrossDeviceError() error {
	return &os.LinkError{Op: "rename", Old: "src", New: "dst", Err: syscall.EXDEV}
}
