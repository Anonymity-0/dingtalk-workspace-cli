// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package app

import (
	"reflect"
	"testing"
)

func TestCrossPlatformCoverageMinutesExportSanitizationFinalSchema(t *testing.T) {
	full := executeShortcutSchemaQuery(t, "--cli-path", "minutes +export-pack")
	compact := executeShortcutSchemaQuery(t, "--cli-path", "minutes +export-pack", "--compact")
	if !reflect.DeepEqual(full["result"], compact["result"]) {
		t.Fatal("compact result differs from full result")
	}
	result := schemaContractMap(full["result"])
	properties := schemaContractMap(schemaContractMap(result["data_schema"])["properties"])
	for _, name := range []string{"published", "path", "manifest", "files", "sanitized", "redactionCount", "redactionKinds", "sanitizationScope", "offlineImagesComplete"} {
		if schemaContractString(schemaContractMap(properties[name])["description"]) == "" {
			t.Errorf("missing field %s", name)
		}
	}
	for _, name := range []string{"sha256", "hashVerified", "readbackVerified"} {
		if _, ok := properties[name]; ok {
			t.Errorf("P1 field unexpectedly declared: %s", name)
		}
	}
}
