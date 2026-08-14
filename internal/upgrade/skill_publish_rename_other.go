//go:build !linux && !darwin && !windows

package upgrade

import (
	"fmt"
	"runtime"
)

func renameSkillPathNoReplace(_, _ string) error {
	return fmt.Errorf("当前平台 %s 不支持安全的原子 no-replace rename", runtime.GOOS)
}
