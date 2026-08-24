// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package cmdutil

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCrossPlatformCoverageSuggestSubcommandsRanksNearestAndBoundsResults(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	group := &cobra.Command{Use: "demo", SuggestionsMinimumDistance: 100}
	root.AddCommand(group)
	for _, name := range []string{"aaa", "alpha", "alphi", "alphx"} {
		group.AddCommand(&cobra.Command{Use: name, Run: func(*cobra.Command, []string) {}})
	}
	group.AddCommand(&cobra.Command{Use: "alpho-hidden", Hidden: true, Run: func(*cobra.Command, []string) {}})

	want := []string{"alpha", "alphi", "alphx"}
	if got := SuggestSubcommands(group, "alpho"); !slices.Equal(got, want) {
		t.Fatalf("SuggestSubcommands() = %#v, want nearest %#v", got, want)
	}
}

func TestCrossPlatformCoverageSuggestSubcommandsUsesAliasesAndReviewedSuggestions(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	group := &cobra.Command{Use: "demo", SuggestionsMinimumDistance: 1}
	root.AddCommand(group)
	group.AddCommand(
		&cobra.Command{Use: "canonical", Aliases: []string{"metas"}, Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "reviewed", SuggestFor: []string{"legacy-command"}, Run: func(*cobra.Command, []string) {}},
	)

	if got := SuggestSubcommands(group, "meta"); !slices.Equal(got, []string{"canonical"}) {
		t.Fatalf("alias suggestion = %#v, want canonical command", got)
	}
	if got := SuggestSubcommands(group, "legacy-command"); !slices.Equal(got, []string{"reviewed"}) {
		t.Fatalf("reviewed SuggestFor = %#v", got)
	}
}

func TestCrossPlatformCoverageGroupRunERemainsConciseWithoutSuggestions(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	group := &cobra.Command{Use: "demo"}
	root.AddCommand(group)
	for _, name := range []string{"alpha", "beta", "gamma", "delta"} {
		group.AddCommand(&cobra.Command{Use: name, Run: func(*cobra.Command, []string) {}})
	}

	err := GroupRunE(group, []string{"zzzzz"})
	if err == nil || strings.Contains(err.Error(), "available:") || !strings.Contains(err.Error(), "dws demo --help") {
		t.Fatalf("GroupRunE() error = %v, want concise parent-help guidance", err)
	}
}

func TestCrossPlatformCoverageFormatSubcommandSuggestionHintDefensivelyBoundsInput(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	group := &cobra.Command{Use: "demo"}
	root.AddCommand(group)
	hint := FormatSubcommandSuggestionHint(group, []string{"one", "two", "three", "four"}, "fallback")
	if strings.Count(hint, `"dws demo `) != MaxCommandSuggestions || strings.Contains(hint, "four") {
		t.Fatalf("unbounded formatted hint = %q", hint)
	}
	if hint := FormatSubcommandSuggestionHint(group, []string{"one"}, "fallback"); hint != `Did you mean "dws demo one"? (fallback)` {
		t.Fatalf("single suggestion hint = %q", hint)
	}
}

func TestCrossPlatformCoverageSuggestSubcommandsDefensiveInputs(t *testing.T) {
	if got := SuggestSubcommands(nil, "candidate"); got != nil {
		t.Fatalf("SuggestSubcommands(nil) = %#v, want nil", got)
	}

	group := &cobra.Command{Use: "demo"}
	group.AddCommand(&cobra.Command{
		Use:     "target",
		Aliases: []string{""},
		Run:     func(*cobra.Command, []string) {},
	})
	if got := SuggestSubcommands(group, " \t "); len(got) != 0 {
		t.Fatalf("empty candidate suggestions = %#v", got)
	}
	if got := SuggestSubcommands(group, "target"); !slices.Equal(got, []string{"target"}) {
		t.Fatalf("empty alias changed canonical suggestion = %#v", got)
	}

	blankGroup := &cobra.Command{Use: "demo", SuggestionsMinimumDistance: 100}
	blankGroup.AddCommand(&cobra.Command{Use: "", Run: func(*cobra.Command, []string) {}})
	if got := SuggestSubcommands(blankGroup, "candidate"); len(got) != 0 {
		t.Fatalf("blank command identity suggestions = %#v", got)
	}
}

func TestCrossPlatformCoverageSuggestSubcommandsRankingTieBreakers(t *testing.T) {
	newGroup := func() *cobra.Command {
		return &cobra.Command{Use: "demo", SuggestionsMinimumDistance: 100}
	}

	explicit := newGroup()
	explicit.AddCommand(
		&cobra.Command{Use: "reviewed", SuggestFor: []string{"legacy"}, Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "ordinary", Run: func(*cobra.Command, []string) {}},
	)
	if got := SuggestSubcommands(explicit, "legacy"); len(got) < 2 || got[0] != "reviewed" {
		t.Fatalf("explicit ranking = %#v", got)
	}

	prefix := newGroup()
	prefix.AddCommand(
		&cobra.Command{Use: "alpha", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "xlp", Run: func(*cobra.Command, []string) {}},
	)
	if got := SuggestSubcommands(prefix, "alp"); len(got) < 2 || got[0] != "alpha" {
		t.Fatalf("prefix ranking = %#v", got)
	}

	lengthDelta := newGroup()
	lengthDelta.AddCommand(
		&cobra.Command{Use: "ab", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "axc", Run: func(*cobra.Command, []string) {}},
	)
	if got := SuggestSubcommands(lengthDelta, "abc"); !slices.Equal(got, []string{"axc", "ab"}) {
		t.Fatalf("length-delta ranking = %#v", got)
	}
}
