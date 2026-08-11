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
		return nil, false, fmt.Errorf("Skill 状态不可读: %w", err)
	}
	return &state, true, nil
}

func Write(home string, state State) error {
	state.OfficialSkills = uniqueSorted(state.OfficialSkills)
	state.UpdatedSkills = uniqueSorted(state.UpdatedSkills)
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := helpers.AtomicWriteJSON(Path(home), append(data, '\n')); err != nil {
		return fmt.Errorf("保存 Skill 状态失败: %w", err)
	}
	return nil
}

func Remove(home string) error {
	if err := skillStateRemove(Path(home)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理 Skill 状态失败: %w", err)
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
