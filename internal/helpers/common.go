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

package helpers

import (
	"path/filepath"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func writeCommandPayload(cmd *cobra.Command, payload any) error {
	return output.WriteCommandPayload(cmd, payload, output.FormatJSON)
}

func preferLegacyLeaf(cmd *cobra.Command) {
	cli.SetOverridePriority(cmd, 100)
}

func commandDryRun(cmd *cobra.Command) bool {
	return commandBoolFlag(cmd, "dry-run")
}

func commandBoolFlag(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	rootFlags := cmd.Root().PersistentFlags()
	for _, flags := range []*pflag.FlagSet{cmd.Flags(), cmd.InheritedFlags(), rootFlags} {
		flag := flags.Lookup(name)
		if flag == nil {
			continue
		}
		value, err := flags.GetBool(name)
		if err == nil {
			return value
		}
	}
	return false
}

// renameBaseName removes one familiar file extension before calling
// rename_document. That service preserves the node's current extension, so
// forwarding "report.txt" for a .txt file would otherwise produce
// "report.txt.txt". Unknown dotted suffixes are preserved because they may be
// part of a display name (for example "release.v2").
func renameBaseName(name string) string {
	trimmed := strings.TrimSpace(name)
	extension := strings.ToLower(strings.TrimPrefix(filepath.Ext(trimmed), "."))
	if _, ok := renamePreservedExtensions[extension]; !ok {
		return trimmed
	}
	return strings.TrimSuffix(trimmed, filepath.Ext(trimmed))
}

var renamePreservedExtensions = map[string]struct{}{
	"7z": {}, "able": {}, "adoc": {}, "adraw": {}, "aform": {}, "amind": {},
	"appt": {}, "awbd": {}, "axls": {}, "csv": {}, "doc": {}, "docx": {},
	"gif": {}, "gz": {}, "jpeg": {}, "jpg": {}, "json": {}, "md": {},
	"mp3": {}, "mp4": {}, "pdf": {}, "png": {}, "ppt": {}, "pptx": {},
	"rar": {}, "svg": {}, "tar": {}, "txt": {}, "wav": {}, "webp": {},
	"xls": {}, "xlsx": {}, "xml": {}, "zip": {},
}
