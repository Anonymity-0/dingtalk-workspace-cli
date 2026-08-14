//go:build windows

package upgrade

import "golang.org/x/sys/windows"

func renameSkillPathNoReplace(source, destination string) error {
	sourcePtr, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	destinationPtr, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFile(sourcePtr, destinationPtr)
}
