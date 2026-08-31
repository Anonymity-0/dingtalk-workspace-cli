//go:build windows

package runtimecontext

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/windows"
)

const initSymbol = "k9Xm2pQv"

func startNative(path string, callback func([]byte, int32, error)) (session nativeSession, accepted int32, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: %v", errNativeBinding, recovered)
		}
	}()
	handle, err := windows.LoadLibrary(path)
	if err != nil {
		return nativeSession{}, 0, fmt.Errorf("%w: %v", errNativeLoad, err)
	}
	session.handle = uintptr(handle)
	symbol, err := windows.GetProcAddress(handle, initSymbol)
	if err != nil {
		return session, 0, fmt.Errorf("%w: %v", errNativeSymbol, err)
	}
	callbackPointer := purego.NewCallback(func(_ purego.CDecl, token unsafe.Pointer, code int32) uintptr {
		raw, copyErr := copyCString(token)
		callback(raw, code, copyErr)
		return 0
	})
	session.callback = callbackPointer
	var initialize func(purego.CDecl, int32, uintptr) int32
	purego.RegisterFunc(&initialize, symbol)
	accepted = initialize(purego.CDecl{}, initializationEnvironment, callbackPointer)
	return session, accepted, nil
}
