//go:build windows

package helpers

import "path/filepath"

// isSafeRemoteSegmentPlatform 承载 Windows 专有的单层名称校验。
//
// 拒绝含盘符的名称（"C:"、"C:foo"）——Windows 上 filepath.VolumeName 会识别它们，
// 拼进 rel_path 后可让落盘目标逃逸出本地根目录；并在 Clean 改写名称时拒绝，兜住
// 分隔符过滤之外的非规范形式。
func isSafeRemoteSegmentPlatform(name string) bool {
	if filepath.VolumeName(name) != "" {
		return false
	}
	if filepath.Clean(name) != name {
		return false
	}
	return true
}
