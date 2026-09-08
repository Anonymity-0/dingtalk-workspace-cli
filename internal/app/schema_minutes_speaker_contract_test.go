// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package app

import (
	"reflect"
	"testing"
)

func TestCrossPlatformCoverageMinutesSpeakerFinalSchema(t *testing.T) {
	full := executeShortcutSchemaQuery(t, "--cli-path", "minutes +speaker-insights")
	compact := executeShortcutSchemaQuery(t, "--cli-path", "minutes +speaker-insights", "--compact")
	if !reflect.DeepEqual(full["result"], compact["result"]) {
		t.Fatal("compact result differs")
	}
	result := schemaContractMap(full["result"])
	properties := schemaContractMap(schemaContractMap(result["data_schema"])["properties"])
	for _, name := range []string{"state", "complete", "taskUuid", "taskId", "createStatus", "attempts", "retryable", "recovery", "result"} {
		if schemaContractString(schemaContractMap(properties[name])["description"]) == "" {
			t.Errorf("missing %s", name)
		}
	}
	if full["confirmation"] != "user_required" {
		t.Fatal("confirmation changed")
	}
}
