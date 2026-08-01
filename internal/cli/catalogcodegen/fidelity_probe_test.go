// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package catalogcodegen_test

import (
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
)

// TestToolSpecIsExpressibleAsLiterals decides whether the catalog can be stored
// as compiled-in Go literals at all.
//
// Byte-identical `dws schema` output requires that a ToolSpec built by generated
// code in another package can hold exactly the state the JSON path produces. That
// is possible if and only if every field reachable from ToolSpec is exported —
// an unexported field cannot be set from outside internal/cli, so its value could
// never be reproduced. This walks the type graph and reports any that block it.
func TestToolSpecIsExpressibleAsLiterals(t *testing.T) {
	var blockers []string
	seen := map[reflect.Type]bool{}

	var walk func(t reflect.Type, path string)
	walk = func(rt reflect.Type, path string) {
		for rt.Kind() == reflect.Ptr || rt.Kind() == reflect.Slice || rt.Kind() == reflect.Array {
			rt = rt.Elem()
		}
		if rt.Kind() == reflect.Map {
			walk(rt.Key(), path+"[key]")
			rt = rt.Elem()
			for rt.Kind() == reflect.Ptr || rt.Kind() == reflect.Slice {
				rt = rt.Elem()
			}
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		seen[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			field := rt.Field(i)
			name := path + "." + field.Name
			if !unicode.IsUpper(rune(field.Name[0])) {
				// json.RawMessage and time internals are stdlib types a literal
				// can still produce by value; only our own types matter.
				if strings.HasPrefix(rt.PkgPath(), "github.com/DingTalk-Real-AI") {
					blockers = append(blockers, name+" ("+rt.String()+")")
				}
				continue
			}
			walk(field.Type, name)
		}
	}
	walk(reflect.TypeOf(cli.ToolSpec{}), "ToolSpec")

	if len(blockers) != 0 {
		t.Fatalf("unexported fields block literal construction: %v", blockers)
	}
	t.Logf("ToolSpec graph is fully exported across %d struct types", len(seen))
}
