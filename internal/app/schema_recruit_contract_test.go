// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package app

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func recruitSchemaLeaf(t *testing.T, canonical string, compact bool) map[string]any {
	t.Helper()
	root := NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	args := []string{"schema", canonical, "--format", "json"}
	if compact {
		args = append(args, "--compact")
	}
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute schema %s compact=%v: %v; stderr=%s", canonical, compact, err, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode schema %s compact=%v: %v", canonical, compact, err)
	}
	return payload
}

func TestRecruitDeliveredSchemaPublishesResultAndPagination(t *testing.T) {
	for _, canonical := range []string{"recruit.list_jobs", "recruit.get_job_detail", "recruit.create_job"} {
		t.Run(canonical, func(t *testing.T) {
			full := recruitSchemaLeaf(t, canonical, false)
			compact := recruitSchemaLeaf(t, canonical, true)
			if full["result"] == nil || compact["result"] == nil {
				t.Fatalf("result missing: full=%#v compact=%#v", full["result"], compact["result"])
			}
			if !reflect.DeepEqual(full["result"], compact["result"]) {
				t.Fatalf("full/compact result mismatch\nfull=%#v\ncompact=%#v", full["result"], compact["result"])
			}
			if canonical == "recruit.list_jobs" {
				if full["pagination"] == nil || compact["pagination"] == nil {
					t.Fatalf("list pagination missing: full=%#v compact=%#v", full["pagination"], compact["pagination"])
				}
				if !reflect.DeepEqual(full["pagination"], compact["pagination"]) {
					t.Fatalf("full/compact pagination mismatch\nfull=%#v\ncompact=%#v", full["pagination"], compact["pagination"])
				}
				parameters, _ := full["parameters"].(map[string]any)
				cursor, _ := parameters["cursor"].(map[string]any)
				if cursor["type"] != "string" || cursor["interface_type"] != "number" {
					t.Fatalf("cursor contract = %#v, want CLI string converted to MCP number", cursor)
				}
			} else if full["pagination"] != nil || compact["pagination"] != nil {
				t.Fatalf("non-list pagination must be absent: full=%#v compact=%#v", full["pagination"], compact["pagination"])
			}
		})
	}
}
