// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interfacesnapshot

import "testing"

func TestCrossPlatformCoverageCompareAdmissionPolicy(t *testing.T) {
	base := testSnapshot(
		testCommand("dws"),
		testCommand("dws search", testFlag("query", "string", false)),
	)

	tests := []struct {
		name       string
		current    Snapshot
		compatible bool
		kind       string
	}{
		{
			name: "command addition allowed",
			current: testSnapshot(
				testCommand("dws"),
				testCommand("dws search", testFlag("query", "string", false)),
				testCommand("dws status"),
			),
			compatible: true,
		},
		{
			name: "flag addition allowed",
			current: testSnapshot(
				testCommand("dws"),
				testCommand("dws search",
					testFlag("limit", "int", false),
					testFlag("query", "string", false),
				),
			),
			compatible: true,
		},
		{
			name: "required flag addition blocked on existing command",
			current: testSnapshot(
				testCommand("dws"),
				testCommand("dws search",
					testFlag("query", "string", false),
					testFlag("tenant", "string", true),
				),
			),
			kind: "required_flag_added",
		},
		{
			name: "command deletion blocked",
			current: testSnapshot(
				testCommand("dws"),
			),
			kind: "command_removed",
		},
		{
			name: "rename without alias blocked",
			current: testSnapshot(
				testCommand("dws"),
				testCommand("dws find", testFlag("query", "string", false)),
			),
			kind: "command_removed",
		},
		{
			name: "flag deletion blocked",
			current: testSnapshot(
				testCommand("dws"),
				testCommand("dws search"),
			),
			kind: "flag_removed",
		},
		{
			name: "optional becoming required blocked",
			current: testSnapshot(
				testCommand("dws"),
				testCommand("dws search", testFlag("query", "string", true)),
			),
			kind: "flag_became_required",
		},
		{
			name: "flag type change blocked",
			current: testSnapshot(
				testCommand("dws"),
				testCommand("dws search", testFlag("query", "stringSlice", false)),
			),
			kind: "flag_type_changed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			comparison := Compare(test.current, base, "base")
			if comparison.Compatible != test.compatible {
				t.Fatalf("Compatible = %v, want %v; blocking=%#v", comparison.Compatible, test.compatible, comparison.Blocking)
			}
			if test.kind == "" {
				return
			}
			if !hasChangeKind(comparison.Blocking, test.kind) {
				t.Fatalf("blocking=%#v, want kind %q", comparison.Blocking, test.kind)
			}
		})
	}
}

func TestCrossPlatformCoverageCompareAllowsRenameWhenOldPathIsAlias(t *testing.T) {
	base := testSnapshot(
		testCommand("dws"),
		testCommand("dws search", testFlag("query", "string", false)),
	)
	current := testSnapshot(
		testCommand("dws"),
		testCommandWithAliases("dws find", []string{"search"}, testFlag("query", "string", false)),
	)

	comparison := Compare(current, base, "base")
	if !comparison.Compatible {
		t.Fatalf("rename with compatibility alias was blocked: %#v", comparison.Blocking)
	}
}

func TestCrossPlatformCoverageCompareBlocksRemovedAlias(t *testing.T) {
	base := testSnapshot(
		testCommand("dws"),
		testCommandWithAliases("dws search", []string{"find"}),
	)
	current := testSnapshot(
		testCommand("dws"),
		testCommand("dws search"),
	)

	comparison := Compare(current, base, "base")
	if comparison.Compatible || !hasChangeKind(comparison.Blocking, "command_alias_removed") {
		t.Fatalf("removed alias was not blocked: %#v", comparison)
	}
}

func TestCrossPlatformCoverageCompareBlocksAliasRetargetedToIncompatibleCommand(t *testing.T) {
	base := testSnapshot(
		testCommand("dws"),
		testCommandWithAliases("dws search", []string{"find"}, testFlag("query", "string", false)),
	)
	current := testSnapshot(
		testCommand("dws"),
		testCommand("dws search", testFlag("query", "string", false)),
		testCommand("dws find"),
	)

	comparison := Compare(current, base, "base")
	if comparison.Compatible || !hasFlagChange(comparison.Blocking, "flag_removed", "dws find", "query") {
		t.Fatalf("alias retarget was not checked against its old contract: %#v", comparison)
	}
}

func TestCrossPlatformCoverageCompareBlocksSnapshotRuleChanges(t *testing.T) {
	base := testSnapshot(testCommand("dws"))
	current := testSnapshot(testCommand("dws"))
	current.Rules.ExcludedFlags = append(current.Rules.ExcludedFlags, "legacy")

	comparison := Compare(current, base, "base")
	if comparison.Compatible || !hasChangeKind(comparison.Blocking, "snapshot_rules_changed") {
		t.Fatalf("snapshot rule change was not blocked: %#v", comparison)
	}
}

func TestCrossPlatformCoverageCompareBlocksCallableMetadataRegressions(t *testing.T) {
	baseFlag := testFlag("format", "string", false)
	baseFlag.Shorthand = "f"
	baseFlag.NoOpt = "json"
	base := testSnapshot(
		testCommand("dws"),
		testCommand("dws export", baseFlag),
	)

	tests := []struct {
		name    string
		command Command
		kind    string
	}{
		{
			name: "command became non-runnable",
			command: func() Command {
				command := testCommand("dws export", baseFlag)
				command.Runnable = false
				return command
			}(),
			kind: "command_became_non_runnable",
		},
		{
			name: "command became hidden",
			command: func() Command {
				command := testCommand("dws export", baseFlag)
				command.Hidden = true
				return command
			}(),
			kind: "command_became_hidden",
		},
		{
			name: "flag shorthand removed",
			command: func() Command {
				flag := baseFlag
				flag.Shorthand = ""
				return testCommand("dws export", flag)
			}(),
			kind: "flag_shorthand_changed",
		},
		{
			name: "flag no-opt behavior removed",
			command: func() Command {
				flag := baseFlag
				flag.NoOpt = ""
				return testCommand("dws export", flag)
			}(),
			kind: "flag_no_opt_changed",
		},
		{
			name: "flag became hidden",
			command: func() Command {
				flag := baseFlag
				flag.Hidden = true
				return testCommand("dws export", flag)
			}(),
			kind: "flag_became_hidden",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := testSnapshot(testCommand("dws"), test.command)
			comparison := Compare(current, base, "base")
			if comparison.Compatible || !hasChangeKind(comparison.Blocking, test.kind) {
				t.Fatalf("metadata regression %q was not blocked: %#v", test.kind, comparison)
			}
		})
	}
}

func TestCrossPlatformCoverageCompareBlocksFlagAliasTargetRegression(t *testing.T) {
	baseLegacy := testFlag("legacy-id", "string", false)
	baseLegacy.AliasOf = "message-id"
	base := testSnapshot(
		testCommand("dws"),
		testCommand("dws send", baseLegacy, testFlag("message-id", "string", false)),
	)

	for _, target := range []string{"", "other-id"} {
		t.Run("target_"+target, func(t *testing.T) {
			currentLegacy := baseLegacy
			currentLegacy.AliasOf = target
			current := testSnapshot(
				testCommand("dws"),
				testCommand(
					"dws send",
					currentLegacy,
					testFlag("message-id", "string", false),
					testFlag("other-id", "string", false),
				),
			)
			comparison := Compare(current, base, "base")
			if comparison.Compatible || !hasFlagChange(
				comparison.Blocking,
				"flag_alias_target_changed",
				"dws send",
				"legacy-id",
			) {
				t.Fatalf("alias target regression was not blocked: %#v", comparison)
			}
		})
	}
}

func TestCrossPlatformCoverageCompareUsesEffectiveFlagsAtEveryCommandPath(t *testing.T) {
	base := testSnapshot(
		testCommand("dws", testFlag("profile", "string", false)),
		testCommandWithFlagScopes(
			"dws search",
			[]Flag{testFlag("query", "string", false)},
			[]Flag{testFlag("profile", "string", false)},
		),
	)

	t.Run("persistent flag scope narrowing is blocked", func(t *testing.T) {
		current := testSnapshot(
			testCommand("dws", testFlag("profile", "string", false)),
			testCommand("dws search", testFlag("query", "string", false)),
		)
		comparison := Compare(current, base, "base")
		if comparison.Compatible || !hasFlagChange(comparison.Blocking, "flag_removed", "dws search", "profile") {
			t.Fatalf("lost inherited flag was not blocked: %#v", comparison)
		}
	})

	t.Run("local flag moved to inherited remains compatible", func(t *testing.T) {
		current := testSnapshot(
			testCommand("dws",
				testFlag("profile", "string", false),
				testFlag("query", "string", false),
			),
			testCommandWithFlagScopes(
				"dws search",
				nil,
				[]Flag{
					testFlag("profile", "string", false),
					testFlag("query", "string", false),
				},
			),
		)
		comparison := Compare(current, base, "base")
		if !comparison.Compatible {
			t.Fatalf("flag moved to an inherited scope was blocked: %#v", comparison.Blocking)
		}
	})
}

func TestCrossPlatformCoverageCompareAllRequiresBothReferencesToPass(t *testing.T) {
	current := testSnapshot(testCommand("dws"), testCommand("dws status"))
	mergeBase := testSnapshot(testCommand("dws"))
	stable := testSnapshot(testCommand("dws"), testCommand("dws legacy"))

	report := CompareAll(current, map[string]Snapshot{
		"stable":     stable,
		"merge-base": mergeBase,
	})
	if report.Compatible {
		t.Fatal("aggregate report passed even though stable comparison removed a command")
	}
	if len(report.Comparisons) != 2 || report.Comparisons[0].Reference != "merge-base" || report.Comparisons[1].Reference != "stable" {
		t.Fatalf("comparisons are not deterministic: %#v", report.Comparisons)
	}
	if !report.Comparisons[0].Compatible || report.Comparisons[1].Compatible {
		t.Fatalf("unexpected per-reference results: %#v", report.Comparisons)
	}
}

func TestCrossPlatformCoverageCompareAllWithFlagMigrationsAuthorizesOnlyTheExactTransition(t *testing.T) {
	before := testFlagMigrationSnapshot(false, true)
	authority := testFlagMigrationManifest(FlagMigrationPending)
	candidate := testFlagMigrationManifest(FlagMigrationConsumed)

	report, err := CompareAllWithFlagMigrations(
		testFlagMigrationSnapshot(true, true),
		map[string]Snapshot{
			"merge-base": before,
			"stable":     before,
		},
		authority,
		candidate,
	)
	if err != nil {
		t.Fatalf("exact base-owned migration was rejected: %v", err)
	}
	if !report.Compatible {
		t.Fatalf("exact migration remained incompatible: %#v", report.Comparisons)
	}
	for _, comparison := range report.Comparisons {
		if len(comparison.Blocking) != 0 {
			t.Fatalf("reference %q retained blocking changes: %#v", comparison.Reference, comparison.Blocking)
		}
	}
}

func TestCrossPlatformCoverageCompareAllWithFlagMigrationsRejectsInvalidLifecycleTransitions(t *testing.T) {
	empty := testEmptyFlagMigrationManifest()
	pending := testFlagMigrationManifest(FlagMigrationPending)
	consumed := testFlagMigrationManifest(FlagMigrationConsumed)
	before := testFlagMigrationSnapshot(false, true)
	after := testFlagMigrationSnapshot(true, true)

	partial := testFlagMigrationSnapshot(false, true)
	partial.Commands[1].LocalFlags[0].Hidden = true
	partial.Commands[1].LocalFlags[0].Required = false
	partial.Commands[1].LocalFlags[0].AliasOf = "message-id"

	typeDrift := testFlagMigrationSnapshot(true, true)
	typeDrift.Commands[1].LocalFlags[0].Type = "stringSlice"
	noOptDrift := testFlagMigrationSnapshot(true, true)
	noOptDrift.Commands[1].LocalFlags[0].NoOpt = ""
	shorthandDrift := testFlagMigrationSnapshot(true, true)
	shorthandDrift.Commands[1].LocalFlags[0].Shorthand = "x"

	tests := []struct {
		name      string
		current   Snapshot
		authority FlagMigrationManifest
		candidate FlagMigrationManifest
	}{
		{
			name:      "candidate-added pending record cannot approve its own surface change",
			current:   after,
			authority: empty,
			candidate: pending,
		},
		{
			name:      "candidate-added consumed record cannot approve its own surface change",
			current:   after,
			authority: empty,
			candidate: consumed,
		},
		{
			name:      "partial migration is rejected",
			current:   partial,
			authority: pending,
			candidate: consumed,
		},
		{
			name:      "pending record cannot be falsely consumed before the surface changes",
			current:   before,
			authority: pending,
			candidate: consumed,
		},
		{
			name:      "completed surface change must consume its pending record",
			current:   after,
			authority: pending,
			candidate: pending,
		},
		{
			name:      "legacy type drift is outside the approval",
			current:   typeDrift,
			authority: pending,
			candidate: consumed,
		},
		{
			name:      "legacy no-opt drift is outside the approval",
			current:   noOptDrift,
			authority: pending,
			candidate: consumed,
		},
		{
			name:      "legacy shorthand drift is outside the approval",
			current:   shorthandDrift,
			authority: pending,
			candidate: consumed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report, err := CompareAllWithFlagMigrations(
				test.current,
				map[string]Snapshot{
					"merge-base": before,
					"stable":     before,
				},
				test.authority,
				test.candidate,
			)
			if err == nil {
				t.Fatalf("invalid transition passed: %#v", report)
			}
		})
	}
}

func TestCrossPlatformCoverageCompareAllWithFlagMigrationsAllowsApprovalCreationBeforeSurfaceChange(t *testing.T) {
	before := testFlagMigrationSnapshot(false, true)
	report, err := CompareAllWithFlagMigrations(
		before,
		map[string]Snapshot{
			"merge-base": before,
			"stable":     before,
		},
		testEmptyFlagMigrationManifest(),
		testFlagMigrationManifest(FlagMigrationPending),
	)
	if err != nil {
		t.Fatalf("pending approval creation was rejected before the surface changed: %v", err)
	}
	if !report.Compatible {
		t.Fatalf("pending approval creation changed compatibility: %#v", report.Comparisons)
	}
}

func TestCrossPlatformCoverageCompareAllWithFlagMigrationsKeepsUnrelatedBreakageBlocking(t *testing.T) {
	before := testFlagMigrationSnapshot(false, true)
	pending := testFlagMigrationManifest(FlagMigrationPending)
	consumed := testFlagMigrationManifest(FlagMigrationConsumed)

	tests := []struct {
		name   string
		mutate func(*Snapshot)
		kind   string
		flag   string
	}{
		{
			name: "unrelated flag removal",
			mutate: func(snapshot *Snapshot) {
				snapshot.Commands[1].LocalFlags = snapshot.Commands[1].LocalFlags[:2]
			},
			kind: "flag_removed",
			flag: "format",
		},
		{
			name: "unrelated flag type change",
			mutate: func(snapshot *Snapshot) {
				snapshot.Commands[1].LocalFlags[2].Type = "stringSlice"
			},
			kind: "flag_type_changed",
			flag: "format",
		},
		{
			name: "unrelated flag no-opt change",
			mutate: func(snapshot *Snapshot) {
				snapshot.Commands[1].LocalFlags[2].NoOpt = ""
			},
			kind: "flag_no_opt_changed",
			flag: "format",
		},
		{
			name: "unrelated flag shorthand change",
			mutate: func(snapshot *Snapshot) {
				snapshot.Commands[1].LocalFlags[2].Shorthand = ""
			},
			kind: "flag_shorthand_changed",
			flag: "format",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := testFlagMigrationSnapshot(true, true)
			test.mutate(&current)
			report, err := CompareAllWithFlagMigrations(
				current,
				map[string]Snapshot{
					"merge-base": before,
					"stable":     before,
				},
				pending,
				consumed,
			)
			if err != nil {
				t.Fatalf("valid migration with unrelated breakage returned a lifecycle error: %v", err)
			}
			if report.Compatible {
				t.Fatalf("unrelated breakage was hidden by migration approval: %#v", report)
			}
			for _, comparison := range report.Comparisons {
				if !hasFlagChange(comparison.Blocking, test.kind, "dws chat send", test.flag) {
					t.Fatalf("reference %q blocking=%#v, want %s for --%s", comparison.Reference, comparison.Blocking, test.kind, test.flag)
				}
			}
		})
	}
}

func TestCrossPlatformCoverageCompareAllWithFlagMigrationsRetainsConsumedReceiptUntilStableCatchesUp(t *testing.T) {
	before := testFlagMigrationSnapshot(false, true)
	after := testFlagMigrationSnapshot(true, true)
	consumed := testFlagMigrationManifest(FlagMigrationConsumed)

	report, err := CompareAllWithFlagMigrations(
		after,
		map[string]Snapshot{
			"merge-base": after,
			"stable":     before,
		},
		consumed,
		consumed,
	)
	if err != nil {
		t.Fatalf("consumed receipt was rejected while stable still needed it: %v", err)
	}
	if !report.Compatible {
		t.Fatalf("stable comparison did not honor the consumed receipt: %#v", report.Comparisons)
	}
}

func TestCrossPlatformCoverageCompareAllWithFlagMigrationsRequiresCleanupAfterAllReferencesCatchUp(t *testing.T) {
	after := testFlagMigrationSnapshot(true, true)
	consumed := testFlagMigrationManifest(FlagMigrationConsumed)
	references := map[string]Snapshot{
		"merge-base": after,
		"stable":     after,
	}

	if report, err := CompareAllWithFlagMigrations(after, references, consumed, consumed); err == nil {
		t.Fatalf("stale consumed receipt was accepted after every reference caught up: %#v", report)
	}

	report, err := CompareAllWithFlagMigrations(after, references, consumed, testEmptyFlagMigrationManifest())
	if err != nil {
		t.Fatalf("cleanup of stale consumed receipt was rejected: %v", err)
	}
	if !report.Compatible {
		t.Fatalf("cleanup-only candidate was incompatible: %#v", report.Comparisons)
	}
}

func testSnapshot(commands ...Command) Snapshot {
	return Snapshot{
		SchemaVersion: SchemaVersion,
		Rules: Rules{
			ExcludedCommandSubtrees: append([]string(nil), excludedCommandSubtrees...),
			ExcludedFlags:           []string{"help"},
		},
		Commands: commands,
	}
}

func testCommand(path string, flags ...Flag) Command {
	return testCommandWithAliases(path, nil, flags...)
}

func testCommandWithAliases(path string, aliases []string, flags ...Flag) Command {
	return Command{
		Path:           path,
		Runnable:       true,
		Aliases:        aliases,
		LocalFlags:     flags,
		InheritedFlags: []Flag{},
	}
}

func testCommandWithFlagScopes(path string, local, inherited []Flag) Command {
	return Command{
		Path:           path,
		Aliases:        []string{},
		LocalFlags:     local,
		InheritedFlags: inherited,
	}
}

func testFlag(name, flagType string, required bool) Flag {
	return Flag{Name: name, Type: flagType, Required: required}
}

func testFlagMigrationSnapshot(after, includeUnrelated bool) Snapshot {
	legacy := Flag{
		Name:      "legacy-id",
		Shorthand: "l",
		Type:      "string",
		NoOpt:     "auto",
		Required:  true,
	}
	flags := []Flag{legacy}
	if after {
		legacy.Required = false
		legacy.Hidden = true
		legacy.AliasOf = "message-id"
		flags = []Flag{
			legacy,
			{
				Name:     "message-id",
				Type:     "string",
				Required: true,
			},
		}
	}
	if includeUnrelated {
		flags = append(flags, Flag{
			Name:      "format",
			Shorthand: "f",
			Type:      "string",
			NoOpt:     "json",
		})
	}
	return testSnapshot(
		testCommand("dws"),
		testCommand("dws chat send", flags...),
	)
}

func testFlagMigrationManifest(state string) FlagMigrationManifest {
	return FlagMigrationManifest{
		Version: FlagMigrationManifestVersion,
		Migrations: []FlagMigration{
			{
				Command: "dws chat send",
				Legacy: FlagMigrationSide{
					Name: "legacy-id",
					Before: FlagMigrationState{
						Present:   true,
						Type:      "string",
						Required:  true,
						Shorthand: "l",
						NoOpt:     "auto",
						Scope:     "local",
					},
					After: FlagMigrationState{
						Present:   true,
						Type:      "string",
						Hidden:    true,
						Shorthand: "l",
						NoOpt:     "auto",
						Scope:     "local",
						AliasOf:   "message-id",
					},
				},
				Canonical: FlagMigrationSide{
					Name:   "message-id",
					Before: FlagMigrationState{},
					After: FlagMigrationState{
						Present:  true,
						Type:     "string",
						Required: true,
						Scope:    "local",
					},
				},
				State:  state,
				Reason: "rename the public flag while preserving the executable legacy alias",
			},
		},
	}
}

func testEmptyFlagMigrationManifest() FlagMigrationManifest {
	return FlagMigrationManifest{
		Version:    FlagMigrationManifestVersion,
		Migrations: []FlagMigration{},
	}
}

func hasChangeKind(changes []Change, kind string) bool {
	for _, change := range changes {
		if change.Kind == kind {
			return true
		}
	}
	return false
}

func hasFlagChange(changes []Change, kind, path, flag string) bool {
	for _, change := range changes {
		if change.Kind == kind && change.Path == path && change.Flag == flag {
			return true
		}
	}
	return false
}
