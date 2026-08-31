// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package runtimecontext

import (
	"errors"
	"unsafe"
)

func copyCString(pointer unsafe.Pointer) ([]byte, error) {
	if pointer == nil {
		return nil, errors.New("nil runtime value")
	}
	bytes := unsafe.Slice((*byte)(pointer), maxHeaderBytes+1)
	for index, value := range bytes {
		if value == 0 {
			result := make([]byte, index)
			copy(result, bytes[:index])
			return result, nil
		}
	}
	return nil, errors.New("unterminated runtime value")
}
