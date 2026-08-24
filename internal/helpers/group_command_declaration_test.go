// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package helpers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestCrossPlatformCoverageGroupRunEDeclarationsUseExplicitGroupConstructor(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read helpers directory: %v", err)
	}

	fset := token.NewFileSet()
	var violations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		// 生成文件必须通过其生成器维护，不能由机械迁移直接改写。
		if ast.IsGenerated(file) {
			continue
		}

		wrapped := explicitGroupCommandLiterals(file)
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok || !declaresGroupRunE(literal) {
				return true
			}
			if _, ok := wrapped[literal]; !ok {
				position := fset.Position(literal.Pos())
				violations = append(violations, position.String())
			}
			return true
		})
	}

	if len(violations) == 0 {
		return
	}
	sort.Strings(violations)
	shown := violations
	if len(shown) > 10 {
		shown = shown[:10]
	}
	t.Fatalf("%d groupRunE declarations bypass newGroupCommand; first %d:\n%s",
		len(violations), len(shown), strings.Join(shown, "\n"))
}

func explicitGroupCommandLiterals(file *ast.File) map[*ast.CompositeLit]struct{} {
	wrapped := make(map[*ast.CompositeLit]struct{})
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		callee, ok := call.Fun.(*ast.Ident)
		if !ok || callee.Name != "newGroupCommand" {
			return true
		}
		argument, ok := call.Args[0].(*ast.UnaryExpr)
		if !ok || argument.Op != token.AND {
			return true
		}
		literal, ok := argument.X.(*ast.CompositeLit)
		if ok {
			wrapped[literal] = struct{}{}
		}
		return true
	})
	return wrapped
}

func declaresGroupRunE(literal *ast.CompositeLit) bool {
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, keyOK := field.Key.(*ast.Ident)
		value, valueOK := field.Value.(*ast.Ident)
		if keyOK && valueOK && key.Name == "RunE" && value.Name == "groupRunE" {
			return true
		}
	}
	return false
}
