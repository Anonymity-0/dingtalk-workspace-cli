//go:build !linux && !darwin && !windows

package upgrade

import (
	"fmt"
	"os"
)

func skillPathFileIncarnation(info os.FileInfo) string {
	return fmt.Sprintf("%d:%d:%s", info.ModTime().UnixNano(), info.Size(), info.Mode())
}
