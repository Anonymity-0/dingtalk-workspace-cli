// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package minutesdata

import (
	"reflect"
	"testing"
)

func TestCrossPlatformCoverageMinutesListDisplayProjection(t *testing.T) {
	page := Page{Items: []map[string]any{{"taskUuid": "u1", "creator": "declared creator", "orgName": "source org", "flashUserInfo": map[string]any{"name": "display user", "uid": "must-not-copy", "extra": "must-not-copy"}}}}
	rows, err := ProjectList(page)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if rows[0]["creator"] != "declared creator" || rows[0]["orgName"] != "source org" || !reflect.DeepEqual(rows[0]["flashUserInfo"], map[string]any{"name": "display user"}) {
		t.Fatalf("projection=%#v", rows[0])
	}
	rows[0]["flashUserInfo"].(map[string]any)["name"] = "changed"
	if page.Items[0]["flashUserInfo"].(map[string]any)["name"] != "display user" {
		t.Fatal("projection aliases input object")
	}
	for _, values := range []map[string]any{
		{}, {"orgName": "", "flashUserInfo": map[string]any{"name": ""}},
		{"orgName": 7, "flashUserInfo": "wrong"}, {"flashUserInfo": map[string]any{"name": 9}},
	} {
		values["taskUuid"] = "u2"
		got, err := ProjectList(Page{Items: []map[string]any{values}})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got[0]["orgName"]; ok {
			t.Fatal("invented orgName")
		}
		if _, ok := got[0]["flashUserInfo"]; ok {
			t.Fatal("invented flashUserInfo")
		}
		if _, ok := got[0]["creator"]; ok {
			t.Fatal("invented creator")
		}
	}
}
