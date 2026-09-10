// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package minutes

import (
	"reflect"
	"testing"
)

func TestCrossPlatformCoverageMinutesListDisplayFinalData(t *testing.T) {
	for _, command := range []string{"+list-mine", "+list-shared", "+list-all", "+search"} {
		c := &minutesE2ECaller{responses: map[string][]string{
			"minutes/list_by_keyword_and_time_range": {`{"success":true,"result":{"itemList":[{"taskUuid":"u1","title":"周会","orgName":"source org","flashUserInfo":{"name":"display user","extra":"must-not-copy"}}],"hasNext":false}}`},
		}}
		args := []string{"minutes", command, "--page-all"}
		wantCalls := 1
		if command == "+search" {
			args = append(args, "--query", "周会", "--scope", "all")
		}
		if command == "+search" || command == "+list-all" {
			wantCalls = 2
		}
		p, _, err := runMinutesAlignmentCLI(t, c, args...)
		if err != nil {
			t.Fatal(err)
		}
		rows := p["minutes"].([]any)
		if len(rows) != 1 {
			t.Fatalf("rows=%#v", rows)
		}
		row := rows[0].(map[string]any)
		if row["orgName"] != "source org" || !reflect.DeepEqual(row["flashUserInfo"], map[string]any{"name": "display user"}) || row["creator"] != nil {
			t.Fatalf("%s row=%#v", command, row)
		}
		if len(c.counts) != 1 || c.counts["minutes/list_by_keyword_and_time_range"] != wantCalls {
			t.Fatalf("unexpected extra calls=%v", c.counts)
		}
	}
}
