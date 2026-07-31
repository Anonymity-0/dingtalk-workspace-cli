// Copyright 2026 Alibaba Group
//
// Intentionally empty after the ContractFinal migration: leaf and shortcut
// declarations now live in helpers.DeclareLeafMetadata / Shortcut.Schema, so the
// lookup in schema_hint_decls.go always misses and callers fall through to the
// declaration path. No generator maintains this table.

package cli

var schemaHintDeclsByCanonical = map[string]schemaHintDecl{}
