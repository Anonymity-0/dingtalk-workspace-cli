// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package pipeline

import (
	"fmt"
	"strings"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// validateUnresolvedCommand gives command identity precedence over flag
// parsing. Cobra otherwise falls back to the nearest parent when a child is
// unknown, so a later flag can incorrectly turn an invented command into an
// "unknown flag" error on that parent.
//
// Only explicit, unambiguous command containers are handled here:
//   - the root command, which has no positional contract;
//   - a top-level service followed by an explicit +shortcut token; and
//   - a command explicitly annotated as a group container.
//
// Other unresolved positionals remain Cobra's responsibility so commands that
// legitimately accept positional arguments are not reclassified.
func validateUnresolvedCommand(target *cobra.Command, remaining []string) error {
	if target == nil || len(remaining) == 0 {
		return nil
	}
	candidate := strings.TrimSpace(remaining[0])
	if candidate == "" || candidate == "--" || strings.HasPrefix(candidate, "-") {
		return nil
	}
	// Cobra registers its built-in `help [command]` lazily during ExecuteC,
	// after this pre-parse traversal runs. Preserve that reserved root entry so
	// `dws help auth` reaches Cobra instead of being classified as a typo.
	if target == target.Root() && candidate == "help" {
		return nil
	}

	if isExplicitShortcutCandidate(target, candidate) {
		return unknownShortcutError(target, candidate)
	}
	if target == target.Root() || cmdutil.IsGroup(target) {
		return unknownSubcommandError(target, candidate)
	}
	return nil
}

func isExplicitShortcutCandidate(target *cobra.Command, candidate string) bool {
	if target == nil || !strings.HasPrefix(candidate, "+") {
		return false
	}
	root := target.Root()
	if root == nil || target == root || target.Parent() != root {
		return false
	}
	for _, child := range target.Commands() {
		if strings.HasPrefix(child.Name(), "+") {
			return true
		}
	}
	return false
}

func unknownShortcutError(parent *cobra.Command, candidate string) error {
	helpAction := fmt.Sprintf("Run '%s --help' for the full list", parent.CommandPath())
	listAction := fmt.Sprintf("Run '%s shortcut list --service %s --format json'", parent.Root().Name(), parent.Name())
	suggestions := cmdutil.SuggestSubcommands(parent, candidate)
	return apperrors.NewValidation(
		fmt.Sprintf("unknown shortcut %q for %q", candidate, parent.CommandPath()),
		apperrors.WithReason("unknown_shortcut"),
		apperrors.WithHint(cmdutil.FormatSubcommandSuggestionHint(parent, suggestions, helpAction)),
		apperrors.WithActions(helpAction, listAction),
		apperrors.WithDetails(commandSuggestionDetails(candidate, suggestions)),
	)
}

func unknownSubcommandError(parent *cobra.Command, candidate string) error {
	action := fmt.Sprintf("Run '%s --help' for the full list", parent.CommandPath())
	suggestions := cmdutil.SuggestSubcommands(parent, candidate)
	return apperrors.NewValidation(
		fmt.Sprintf("unknown subcommand %q for %q", candidate, parent.CommandPath()),
		apperrors.WithReason("unknown_subcommand"),
		apperrors.WithHint(cmdutil.FormatSubcommandSuggestionHint(parent, suggestions, action)),
		apperrors.WithActions(action),
		apperrors.WithDetails(commandSuggestionDetails(candidate, suggestions)),
	)
}

func commandSuggestionDetails(candidate string, suggestions []string) map[string]any {
	return map[string]any{
		"input":       candidate,
		"suggestions": suggestions,
	}
}

// HintSubCmd creates a hidden compatibility command for a known wrong path.
// It preserves cmdutil's hint-only identity while returning the same typed
// validation contract as fuzzy unknown-subcommand recovery.
func HintSubCmd(use, authoredHint string) *cobra.Command {
	command := cmdutil.HintSubCmd(use, authoredHint)
	command.RunE = func(cmd *cobra.Command, _ []string) error {
		parent := cmd.Parent()
		if parent == nil {
			parent = cmd.Root()
		}
		parentPath := parent.CommandPath()
		action := fmt.Sprintf("Run '%s --help' for the full list", parentPath)
		hint := strings.TrimSpace(authoredHint)
		if hint == "" {
			hint = action
		} else {
			hint += " (" + action + ")"
		}
		return apperrors.NewValidation(
			fmt.Sprintf("unknown subcommand %q for %q", cmd.Name(), parentPath),
			apperrors.WithReason("unknown_subcommand"),
			apperrors.WithHint(hint),
			apperrors.WithActions(action),
			apperrors.WithDetails(commandSuggestionDetails(cmd.Name(), []string{})),
		)
	}
	return command
}

// GroupRunE is the structured group handler used by the DWS command tree.
func GroupRunE(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return unknownSubcommandError(cmd, strings.TrimSpace(args[0]))
	}
	return cmd.Help()
}
