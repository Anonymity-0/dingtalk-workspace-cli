// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package app

import (
	"reflect"
	"testing"
)

func TestCrossPlatformCoverageMinutesDisplayFinalSchema(t *testing.T) {
	for _, path := range []string{"minutes +list-mine", "minutes +list-shared", "minutes +list-all", "minutes +search"} {
		full := executeShortcutSchemaQuery(t, "--cli-path", path)
		compact := executeShortcutSchemaQuery(t, "--cli-path", path, "--compact")
		if !reflect.DeepEqual(full["result"], compact["result"]) {
			t.Fatalf("%s compact result drift", path)
		}
		props := schemaContractMap(schemaContractMap(schemaContractMap(full["result"])["data_schema"])["properties"])
		item := schemaContractMap(schemaContractMap(props["minutes"])["items"])
		display := schemaContractMap(item["properties"])
		for _, field := range []string{"orgName", "flashUserInfo"} {
			if schemaContractString(schemaContractMap(display[field])["description"]) == "" {
				t.Fatalf("%s missing %s", path, field)
			}
		}
		if full["confirmation"] != "not_required" {
			t.Fatalf("%s confirmation drift", path)
		}
	}
}
