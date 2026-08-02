// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

// Package catalogcodegen is a throwaway feasibility probe: it measures what it
// costs to store one product's Schema catalog as compiled-in Go literals rather
// than embedded JSON, which is how internal/shortcut stores its 376 definitions
// (measured at 1.9ms/3MB versus the full catalog assemble ~294–360ms/~175MB;
// ResolveMeta itself now uses assembled SchemaRegistry instead).
//
// The struct shape mirrors the runtime-consumed subset of schemaToolWire. It is
// structurally equivalent rather than shared, because the real type is
// package-private to internal/cli; the point is to measure how the Go compiler
// and linker handle this data volume.
package catalogcodegen

// Param is the runtime-consumed subset of a catalog parameter.
type Param struct {
	Name         string
	Type         string
	Description  string
	Property     string
	Required     bool
	CLIRequired  bool
	RequiredWhen string
	Format       string
	Enum         []string
}

// Tool is the runtime-consumed subset of a catalog leaf ToolSpec.
type Tool struct {
	Name           string
	CanonicalPath  string
	CLIPath        string
	PrimaryCLIPath string
	ProductID      string
	Group          string
	Title          string
	Description    string
	Aliases        []string
	IsAlias        bool
	Effect         string
	Risk           string
	Confirmation   string
	Idempotency    string
	InterfaceMode  string
	Availability   string
	AgentSummary   string
	UseWhen        []string
	AvoidWhen      []string
	Examples       []string
	HasParameters  bool
	ParameterCount int
	Parameters     map[string]Param
}
