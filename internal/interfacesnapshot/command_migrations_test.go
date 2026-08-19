// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package interfacesnapshot

import (
	"errors"
	"os"
	"strings"
	"testing"
)

type commandMigrationErrorReader struct{}

func (commandMigrationErrorReader) Read([]byte) (int, error) {
	return 0, errors.New("command migration read failure")
}

func TestApprovedCommandMigrationManifestRemainsValid(t *testing.T) {
	manifest, err := os.Open("../../scripts/policy/interface-migrations/approved-command-migrations-v1.json")
	if err != nil {
		t.Fatalf("open approved command migration manifest: %v", err)
	}
	defer manifest.Close()
	if _, err := ReadCommandMigrationManifest(manifest); err != nil {
		t.Fatalf("approved command migration manifest is invalid: %v", err)
	}
}

func TestCrossPlatformCoverageReadCommandMigrationManifestFailsClosed(t *testing.T) {
	valid := commandMigrationManifestJSON()
	if _, err := ReadCommandMigrationManifest(strings.NewReader(valid)); err != nil {
		t.Fatalf("valid command migration manifest: %v", err)
	}
	for _, test := range []struct {
		input string
		want  string
	}{
		{strings.Replace(valid, `"state": "pending"`, `"state": "pending", "allow": true`, 1), "unknown field"},
		{strings.Replace(valid, `"state": "pending"`, `"state": "pending", "state": "consumed"`, 1), "duplicate field"},
		{strings.Replace(valid, `"version": 1`, `"version": null`, 1), "must be"},
		{strings.Replace(valid, "dws chat message old", "dws chat *", 1), "exact command paths"},
		{strings.Replace(valid, `"kind": "command_move"`, `"kind": "anything"`, 1), "invalid kind"},
		{`{"version":1,"migrations":null}`, "must be an array"},
	} {
		if _, err := ReadCommandMigrationManifest(strings.NewReader(test.input)); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("ReadCommandMigrationManifest() error=%v, want %q", err, test.want)
		}
	}
	if _, err := ReadCommandMigrationManifest(commandMigrationErrorReader{}); err == nil || !strings.Contains(err.Error(), "read failure") {
		t.Fatalf("reader error=%v", err)
	}
	for _, input := range []string{valid + ` {}`, valid + ` {`} {
		if _, err := ReadCommandMigrationManifest(strings.NewReader(input)); err == nil || !strings.Contains(err.Error(), "trailing") {
			t.Fatalf("trailing input error=%v", err)
		}
	}
}

func TestCrossPlatformCoverageCommandMigrationValidationEdges(t *testing.T) {
	validMove := commandMigrationManifest(CommandMigrationPending).Migrations[0]
	validExtraction := commandMigrationManifest(CommandMigrationPending).Migrations[1]

	duplicate := CommandMigrationManifest{Version: CommandMigrationManifestVersion, Migrations: []CommandMigration{validMove, validMove}}
	if err := duplicate.Validate(); err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("duplicate error=%v", err)
	}
	if err := (CommandMigrationManifest{Version: CommandMigrationManifestVersion}).Validate(); err == nil || !strings.Contains(err.Error(), "must be an array") {
		t.Fatalf("nil migrations error=%v", err)
	}

	for _, test := range []struct {
		name   string
		mutate func(*CommandMigration)
		want   string
	}{
		{"same command", func(m *CommandMigration) { m.Replacement.Command = m.Legacy.Command }, "must differ"},
		{"empty reason", func(m *CommandMigration) { m.Reason = " " }, "non-empty trimmed"},
		{"invalid state", func(m *CommandMigration) { m.State = "approved" }, "invalid state"},
		{"invalid command state", func(m *CommandMigration) { m.Replacement.Before.Runnable = true }, "absent state"},
		{"legacy contract", func(m *CommandMigration) { m.Legacy.Before.Hidden = true }, "remain runnable"},
		{"replacement contract", func(m *CommandMigration) { m.Replacement.After.Hidden = true }, "absent to visible"},
		{"schema contract", func(m *CommandMigration) { m.Schema.ProductID = "bad/id" }, "exact identifier"},
		{"move not hidden", func(m *CommandMigration) { m.Legacy.After.Hidden = false }, "must migrate exactly"},
		{"move flag", func(m *CommandMigration) { m.LegacyFlag.Name = "thread" }, "must not declare legacy_flag"},
		{"move tool identity", func(m *CommandMigration) { m.Schema.ReplacementToolID = "chat.new" }, "stable Schema tool"},
	} {
		t.Run(test.name, func(t *testing.T) {
			migration := validMove
			test.mutate(&migration)
			if err := migration.validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validate error=%v, want %q", err, test.want)
			}
		})
	}
	for _, test := range []struct {
		name   string
		mutate func(*CommandMigration)
		want   string
	}{
		{"legacy changed", func(m *CommandMigration) { m.Legacy.After.Hidden = true }, "remain visible"},
		{"invalid legacy flag", func(m *CommandMigration) { m.LegacyFlag.Name = "bad name" }, "exact flag"},
		{"same tool", func(m *CommandMigration) { m.Schema.ReplacementToolID = m.Schema.SourceToolID }, "distinct replacement"},
		{"wrong parameter", func(m *CommandMigration) { m.Schema.Parameters[0].From = "other" }, "exact extracted flag"},
	} {
		t.Run(test.name, func(t *testing.T) {
			migration := validExtraction
			migration.Schema.Parameters = append([]CommandParameterMigration(nil), validExtraction.Schema.Parameters...)
			test.mutate(&migration)
			if err := migration.validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validate error=%v, want %q", err, test.want)
			}
		})
	}

	for _, state := range []CommandMigrationState{
		{Runnable: true},
		{Present: true},
	} {
		if err := state.validate("state"); err == nil {
			t.Fatalf("invalid command state accepted: %#v", state)
		}
	}

	for _, test := range []struct {
		name   string
		mutate func(*CommandMigrationFlag)
		want   string
	}{
		{"name", func(f *CommandMigrationFlag) { f.Name = "bad name" }, "exact flag"},
		{"before state", func(f *CommandMigrationFlag) { f.Before = FlagMigrationState{Present: true} }, "requires type"},
		{"after state", func(f *CommandMigrationFlag) { f.After = FlagMigrationState{Present: true} }, "requires type"},
		{"visibility", func(f *CommandMigrationFlag) { f.After.Hidden = false }, "visible to hidden"},
		{"attribute", func(f *CommandMigrationFlag) { f.After.Type = "string" }, "only hidden visibility"},
	} {
		t.Run("flag "+test.name, func(t *testing.T) {
			flag := validExtraction.LegacyFlag
			test.mutate(&flag)
			if err := flag.validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("flag validate error=%v, want %q", err, test.want)
			}
		})
	}

	validSchema := validMove.Schema
	for _, test := range []struct {
		name   string
		kind   string
		mutate func(*CommandMigrationSchema)
		want   string
	}{
		{"identifier", CommandMigrationMove, func(s *CommandMigrationSchema) { s.ProductID = "bad/id" }, "exact identifier"},
		{"nil parameters", CommandMigrationMove, func(s *CommandMigrationSchema) { s.Parameters = nil }, "must be an array"},
		{"bad from", CommandMigrationMove, func(s *CommandMigrationSchema) { s.Parameters[0].From = "bad name" }, "exact parameter"},
		{"duplicate from", CommandMigrationMove, func(s *CommandMigrationSchema) { s.Parameters = append(s.Parameters, s.Parameters[0]) }, "duplicates from"},
		{"bad to", CommandMigrationMove, func(s *CommandMigrationSchema) { s.Parameters[0].To = s.Parameters[0].From }, "distinct exact"},
		{"duplicate to", CommandMigrationMove, func(s *CommandMigrationSchema) {
			s.Parameters = append(s.Parameters, CommandParameterMigration{From: "other", To: s.Parameters[0].To})
		}, "duplicates to"},
		{"extraction to", CommandMigrationFlagExtraction, func(s *CommandMigrationSchema) { s.Parameters[0].To = "new-id" }, "must not declare to"},
	} {
		t.Run("schema "+test.name, func(t *testing.T) {
			schema := validSchema
			schema.Parameters = append([]CommandParameterMigration(nil), validSchema.Parameters...)
			test.mutate(&schema)
			if err := schema.validate(test.kind); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("schema validate error=%v, want %q", err, test.want)
			}
		})
	}
}

func TestCrossPlatformCoverageCommandMigrationLifecycleEdges(t *testing.T) {
	before := commandMigrationSnapshot(false, false)
	after := commandMigrationSnapshot(true, false)
	partial := commandMigrationSnapshot(false, false)
	partial.Commands = append(partial.Commands, testCommand("dws chat topic new"))
	pending := singleCommandMigrationManifest(CommandMigrationPending)
	consumed := singleCommandMigrationManifest(CommandMigrationConsumed)
	empty := CommandMigrationManifest{Version: CommandMigrationManifestVersion, Migrations: []CommandMigration{}}

	if got, err := AuthorizeCommandMigrations(before, map[string]Snapshot{}, empty, empty); err != nil || len(got) != 0 {
		t.Fatalf("empty lifecycle=%#v, %v", got, err)
	}
	for _, test := range []struct {
		name       string
		current    Snapshot
		references map[string]Snapshot
		authority  CommandMigrationManifest
		candidate  CommandMigrationManifest
		want       string
	}{
		{"missing stable", before, map[string]Snapshot{"main": before}, pending, pending, "stable reference"},
		{"missing base", before, map[string]Snapshot{"stable": before}, pending, pending, "main or merge-base"},
		{"pending base after", after, map[string]Snapshot{"main": after, "stable": before}, pending, pending, "want exact before"},
		{"modified approval", before, map[string]Snapshot{"main": before, "stable": before}, pending, modifiedCommandManifest(pending), "modified base-owned"},
		{"pending removed", before, map[string]Snapshot{"main": before, "stable": before}, pending, empty, "removed pending"},
		{"false consumed", before, map[string]Snapshot{"main": before, "stable": before}, pending, consumed, "falsely consumed"},
		{"after pending receipt", after, map[string]Snapshot{"main": before, "stable": before}, pending, pending, "without marking it consumed"},
		{"partial", partial, map[string]Snapshot{"main": before, "stable": before}, pending, consumed, "partially applied"},
		{"consumed drift", before, map[string]Snapshot{"main": after, "stable": before}, consumed, consumed, "drifted from consumed"},
		{"stale receipt", after, map[string]Snapshot{"main": after, "stable": after}, consumed, consumed, "stale after all references"},
		{"early cleanup", after, map[string]Snapshot{"main": after, "stable": before}, consumed, empty, "must retain consumed"},
		{"consumed back to pending", after, map[string]Snapshot{"main": after, "stable": before}, consumed, pending, "must retain consumed"},
		{"candidate added consumed", after, map[string]Snapshot{"main": before, "stable": before}, empty, consumed, "must start pending"},
		{"candidate base mismatch", before, map[string]Snapshot{"main": after, "stable": before}, empty, pending, "does not match"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := AuthorizeCommandMigrations(test.current, test.references, test.authority, test.candidate)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("lifecycle error=%v, want %q", err, test.want)
			}
		})
	}
	if got, err := AuthorizeCommandMigrations(after, map[string]Snapshot{"main": after, "stable": before}, consumed, consumed); err != nil || len(got) != 1 {
		t.Fatalf("retained consumed receipt=%#v, %v", got, err)
	}
	if got, err := AuthorizeCommandMigrations(after, map[string]Snapshot{"main": after, "stable": after}, consumed, empty); err != nil || len(got) != 0 {
		t.Fatalf("cleaned stale receipt=%#v, %v", got, err)
	}
	if got, err := AuthorizeCommandMigrations(before, map[string]Snapshot{"main": before, "stable": before}, empty, pending); err != nil || len(got) != 0 {
		t.Fatalf("candidate pending plan=%#v, %v", got, err)
	}

	invalidSnapshot := before
	invalidSnapshot.SchemaVersion = 0
	for _, test := range []struct {
		name       string
		current    Snapshot
		references map[string]Snapshot
		authority  CommandMigrationManifest
		candidate  CommandMigrationManifest
	}{
		{"current", invalidSnapshot, map[string]Snapshot{"main": before}, empty, empty},
		{"reference", before, map[string]Snapshot{"main": invalidSnapshot}, empty, empty},
		{"authority", before, map[string]Snapshot{"main": before}, CommandMigrationManifest{}, empty},
		{"candidate", before, map[string]Snapshot{"main": before}, empty, CommandMigrationManifest{}},
	} {
		t.Run("invalid "+test.name, func(t *testing.T) {
			if _, err := AuthorizeCommandMigrations(test.current, test.references, test.authority, test.candidate); err == nil {
				t.Fatal("invalid authorization input accepted")
			}
		})
	}
	invalidFlags := FlagMigrationManifest{}
	validFlags := FlagMigrationManifest{Version: FlagMigrationManifestVersion, Migrations: []FlagMigration{}}
	validCommands := CommandMigrationManifest{Version: CommandMigrationManifestVersion, Migrations: []CommandMigration{}}
	if _, err := CompareAllWithInterfaceMigrations(before, map[string]Snapshot{"main": before}, invalidFlags, validFlags, validCommands, validCommands); err == nil {
		t.Fatal("combined compare accepted invalid flag lifecycle")
	}
	if _, err := CompareAllWithInterfaceMigrations(before, map[string]Snapshot{"main": before}, validFlags, validFlags, CommandMigrationManifest{}, validCommands); err == nil {
		t.Fatal("combined compare accepted invalid command lifecycle")
	}
	if commandMigrationAuthorizesChange(before, before, Change{Kind: "command_removed", Path: "dws unrelated"}, pending.Migrations) {
		t.Fatal("unrelated command change was authorized")
	}
}

func TestCrossPlatformCoverageCommandMigrationLifecycleAndExactFiltering(t *testing.T) {
	before := commandMigrationSnapshot(false, false)
	after := commandMigrationSnapshot(true, false)
	pending := commandMigrationManifest(CommandMigrationPending)
	consumed := commandMigrationManifest(CommandMigrationConsumed)
	emptyFlags := FlagMigrationManifest{Version: FlagMigrationManifestVersion, Migrations: []FlagMigration{}}

	report, err := CompareAllWithInterfaceMigrations(
		after,
		map[string]Snapshot{"merge-base": before, "stable": before},
		emptyFlags,
		emptyFlags,
		pending,
		consumed,
	)
	if err != nil {
		t.Fatalf("governed command migration: %v", err)
	}
	if !report.Compatible {
		t.Fatalf("exact command migration remained blocking: %#v", report.Comparisons)
	}

	unrelated := commandMigrationSnapshot(true, true)
	report, err = CompareAllWithInterfaceMigrations(
		unrelated,
		map[string]Snapshot{"merge-base": before, "stable": before},
		emptyFlags,
		emptyFlags,
		pending,
		consumed,
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.Compatible || !hasChangeKind(report.Comparisons[0].Blocking, "flag_removed") {
		t.Fatalf("unrelated flag removal was hidden: %#v", report.Comparisons)
	}

	emptyCommands := CommandMigrationManifest{Version: CommandMigrationManifestVersion, Migrations: []CommandMigration{}}
	if _, err := AuthorizeCommandMigrations(
		after,
		map[string]Snapshot{"merge-base": before, "stable": before},
		emptyCommands,
		pending,
	); err == nil || !strings.Contains(err.Error(), "cannot authorize its own interface change") {
		t.Fatalf("candidate self-authorization error=%v", err)
	}

	partial := commandMigrationSnapshot(true, false)
	partial.Commands = partial.Commands[:len(partial.Commands)-1]
	if _, err := AuthorizeCommandMigrations(
		partial,
		map[string]Snapshot{"merge-base": before, "stable": before},
		pending,
		consumed,
	); err == nil || !strings.Contains(err.Error(), "partially applied") {
		t.Fatalf("partial command migration error=%v", err)
	}
}

func commandMigrationManifestJSON() string {
	return `{
  "version": 1,
  "migrations": [{
    "kind": "command_move",
    "legacy": {
      "command": "dws chat message old",
      "before": {"present": true, "runnable": true},
      "after": {"present": true, "runnable": true, "hidden": true}
    },
    "replacement": {
      "command": "dws chat topic new",
      "before": {"present": false},
      "after": {"present": true, "runnable": true}
    },
    "schema": {
      "product_id": "chat",
      "source_tool_id": "chat.old",
      "replacement_tool_id": "chat.old",
      "parameters": [{"from": "old-id", "to": "new-id"}]
    },
    "state": "pending",
    "reason": "Reviewed command move."
  }]
}`
}

func commandMigrationManifest(state string) CommandMigrationManifest {
	move := CommandMigration{
		Kind: CommandMigrationMove,
		Legacy: CommandMigrationSide{
			Command: "dws chat message old",
			Before:  CommandMigrationState{Present: true, Runnable: true},
			After:   CommandMigrationState{Present: true, Runnable: true, Hidden: true},
		},
		Replacement: CommandMigrationSide{
			Command: "dws chat topic new",
			Before:  CommandMigrationState{},
			After:   CommandMigrationState{Present: true, Runnable: true},
		},
		Schema: CommandMigrationSchema{
			ProductID:         "chat",
			SourceToolID:      "chat.old",
			ReplacementToolID: "chat.old",
			Parameters:        []CommandParameterMigration{{From: "old-id", To: "new-id"}},
		},
		State:  state,
		Reason: "Reviewed command move.",
	}
	extraction := CommandMigration{
		Kind: CommandMigrationFlagExtraction,
		Legacy: CommandMigrationSide{
			Command: "dws chat group create",
			Before:  CommandMigrationState{Present: true, Runnable: true},
			After:   CommandMigrationState{Present: true, Runnable: true},
		},
		Replacement: CommandMigrationSide{
			Command: "dws chat topic create",
			Before:  CommandMigrationState{},
			After:   CommandMigrationState{Present: true, Runnable: true},
		},
		LegacyFlag: CommandMigrationFlag{
			Name:   "thread",
			Before: FlagMigrationState{Present: true, Type: "bool", NoOpt: "true", Scope: "local"},
			After:  FlagMigrationState{Present: true, Type: "bool", Hidden: true, NoOpt: "true", Scope: "local"},
		},
		Schema: CommandMigrationSchema{
			ProductID:         "chat",
			SourceToolID:      "chat.create_group",
			ReplacementToolID: "chat.create_topic",
			Parameters:        []CommandParameterMigration{{From: "thread"}},
		},
		State:  state,
		Reason: "Reviewed flag extraction.",
	}
	return CommandMigrationManifest{Version: CommandMigrationManifestVersion, Migrations: []CommandMigration{move, extraction}}
}

func singleCommandMigrationManifest(state string) CommandMigrationManifest {
	manifest := commandMigrationManifest(state)
	manifest.Migrations = manifest.Migrations[:1]
	return manifest
}

func modifiedCommandManifest(source CommandMigrationManifest) CommandMigrationManifest {
	modified := source
	modified.Migrations = append([]CommandMigration(nil), source.Migrations...)
	modified.Migrations[0].Reason = "Modified reason."
	return modified
}

func commandMigrationSnapshot(after, removeUnrelated bool) Snapshot {
	oldFlags := []Flag{{Name: "old-id", Type: "string", Required: true}, {Name: "keep", Type: "string"}}
	groupFlags := []Flag{{Name: "thread", Type: "bool", NoOpt: "true"}}
	commands := []Command{
		testCommand("dws"),
		testCommand("dws chat group create", groupFlags...),
		testCommand("dws chat message old", oldFlags...),
	}
	if after {
		commands[1].LocalFlags[0].Hidden = true
		commands[2].Hidden = true
		if removeUnrelated {
			commands[2].LocalFlags = commands[2].LocalFlags[:1]
		}
		commands = append(commands,
			testCommand("dws chat topic create"),
			testCommand("dws chat topic new", Flag{Name: "new-id", Type: "string", Required: true}),
		)
	}
	return testSnapshot(commands...)
}
