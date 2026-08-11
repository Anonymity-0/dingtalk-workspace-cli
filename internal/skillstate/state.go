// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

// Package skillstate persists the official Skill snapshot written after setup
// and upgrade. The snapshot is informational; bundled skills are always fully
// refreshed from the current release and local absence is not an exclusion.
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

// legacyOfficialSkillNames is the frozen set of official multi-skill names
// shipped before .dws-managed was introduced. Exact names are safe ownership
// evidence for the marker migration; a dingtalk-* prefix is not.
//
// Keep retired names here permanently so an old installation can still be
// migrated after that Skill has been folded into another bundle.
var legacyOfficialSkillNames = map[string]struct{}{
	"dingtalk-agoal":      {},
	"dingtalk-aiapp":      {},
	"dingtalk-aisearch":   {},
	"dingtalk-aitable":    {},
	"dingtalk-attendance": {},
	"dingtalk-calendar":   {},
	"dingtalk-chat":       {},
	"dingtalk-contact":    {},
	"dingtalk-dev":        {},
	"dingtalk-devapp":     {},
	"dingtalk-devdoc":     {},
	"dingtalk-ding":       {},
	"dingtalk-doc":        {},
	"dingtalk-drive":      {},
	"dingtalk-event":      {},
	"dingtalk-hrbrain":    {},
	"dingtalk-live":       {},
	"dingtalk-mail":       {},
	"dingtalk-markdown":   {},
	"dingtalk-minutes":    {},
	"dingtalk-misc":       {},
	"dingtalk-oa":         {},
	"dingtalk-pat":        {},
	"dingtalk-profile":    {},
	"dingtalk-report":     {},
	"dingtalk-shared":     {},
	"dingtalk-sheet":      {},
	"dingtalk-skill":      {},
	"dingtalk-todo":       {},
	"dingtalk-wiki":       {},
	"dws-shared":          {},
}

var (
	skillStateReadFile = os.ReadFile
	skillStateRemove   = os.Remove
)

type State struct {
	Version        string   `json:"version"`
	OfficialSkills []string `json:"official_skills"`
	UpdatedSkills  []string `json:"updated_skills"`
	UpdatedAt      string   `json:"updated_at"`
}

// IsLegacyOfficialSkillName reports whether name was an exact official
// multi-skill directory name before managed markers were shipped.
func IsLegacyOfficialSkillName(name string) bool {
	_, ok := legacyOfficialSkillNames[name]
	return ok
}

func Path(home string) string {
	if configured := strings.TrimSpace(os.Getenv("DWS_CONFIG_DIR")); configured != "" {
		return filepath.Join(configured, stateFile)
	}
	return filepath.Join(home, ".dws", stateFile)
}

func Read(home string) (*State, bool, error) {
	data, err := skillStateReadFile(Path(home))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, false, fmt.Errorf("skill 状态不可读: %w", err)
	}
	return &state, true, nil
}

func Write(home string, state State) error {
	state.OfficialSkills = uniqueSorted(state.OfficialSkills)
	state.UpdatedSkills = uniqueSorted(state.UpdatedSkills)
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := helpers.AtomicWriteJSON(Path(home), append(data, '\n')); err != nil {
		return fmt.Errorf("保存 skill 状态失败: %w", err)
	}
	return nil
}

func Remove(home string) error {
	if err := skillStateRemove(Path(home)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理 skill 状态失败: %w", err)
	}
	return nil
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
