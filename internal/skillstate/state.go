// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

// Package skillstate persists the official Skill snapshot used by incremental
// updates. Its model follows lark-cli: local presence expresses continued use,
// while additions are derived by comparing consecutive official snapshots.
package skillstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
)

const stateFile = "skills-state.json"

type State struct {
	Version              string   `json:"version"`
	OfficialSkills       []string `json:"official_skills"`
	UpdatedSkills        []string `json:"updated_skills"`
	AddedOfficialSkills  []string `json:"added_official_skills"`
	SkippedDeletedSkills []string `json:"skipped_deleted_skills"`
	UpdatedAt            string   `json:"updated_at"`
}

type SyncInput struct {
	OfficialSkills []string
	LocalSkills    []string
	PreviousState  *State
	StateReadable  bool
	Force          bool
}

type SyncPlan struct {
	OfficialSkills []string
	ToUpdate       []string
	Added          []string
	SkippedDeleted []string
}

func Path(home string) string {
	if configured := strings.TrimSpace(os.Getenv("DWS_CONFIG_DIR")); configured != "" {
		return filepath.Join(configured, stateFile)
	}
	return filepath.Join(home, ".dws", stateFile)
}

func Read(home string) (*State, bool, error) {
	data, err := os.ReadFile(Path(home))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, false, fmt.Errorf("Skill 状态不可读: %w", err)
	}
	return &state, true, nil
}

func Write(home string, state State) error {
	state.OfficialSkills = uniqueSorted(state.OfficialSkills)
	state.UpdatedSkills = uniqueSorted(state.UpdatedSkills)
	state.AddedOfficialSkills = uniqueSorted(state.AddedOfficialSkills)
	state.SkippedDeletedSkills = uniqueSorted(state.SkippedDeletedSkills)
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := helpers.AtomicWriteJSON(Path(home), append(data, '\n')); err != nil {
		return fmt.Errorf("保存 Skill 状态失败: %w", err)
	}
	return nil
}

func Remove(home string) error {
	if err := os.Remove(Path(home)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理 Skill 状态失败: %w", err)
	}
	return nil
}

// Plan applies the lark-cli reconciliation formula:
//
//	(local installed ∩ current official) ∪ (current official - previous official)
func Plan(input SyncInput) SyncPlan {
	official := uniqueSorted(input.OfficialSkills)
	if input.Force {
		return SyncPlan{OfficialSkills: official, ToUpdate: official}
	}
	officialSet := toSet(official)
	updateSet := toSet(intersection(input.LocalSkills, officialSet))
	previousSet := map[string]bool{}
	if input.StateReadable && input.PreviousState != nil {
		previousSet = toSet(input.PreviousState.OfficialSkills)
	}
	var added []string
	for _, name := range official {
		if !previousSet[name] {
			added = append(added, name)
			updateSet[name] = true
		}
	}
	toUpdate := sortedKeys(updateSet)
	updatedSet := toSet(toUpdate)
	var skipped []string
	for _, name := range official {
		if !updatedSet[name] {
			skipped = append(skipped, name)
		}
	}
	return SyncPlan{OfficialSkills: official, ToUpdate: toUpdate, Added: uniqueSorted(added), SkippedDeleted: skipped}
}

func intersection(values []string, allowed map[string]bool) []string {
	var out []string
	for _, value := range uniqueSorted(values) {
		if allowed[value] {
			out = append(out, value)
		}
	}
	return out
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func toSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
