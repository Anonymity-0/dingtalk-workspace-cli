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

package cli

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestDeletedProductionSchemaHintFilesStayGone locks stage-3 cleanup: the
// former production RegisterSchemaHints init files must not return. Their
// pins live on leaf ParamDecl / ContractFinal; RegisterSchemaHints remains
// only for test-fixture injection.
func TestDeletedProductionSchemaHintFilesStayGone(t *testing.T) {
	deleted := []string{
		"schema_hints_aitable.go",
		"schema_hints_attendance.go",
		"schema_hints_ding.go",
		"schema_hints_helper_roots.go",
		"schema_hints_runtime.go",
	}
	for _, name := range deleted {
		path := filepath.Join(".", name)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("%s must stay deleted after RegisterSchemaHints→ParamDecl migration", name)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", name, err)
		}
	}
}

// TestEmbeddedMetadataHintShellsHaveEmptyTools locks the phase-5 end-state:
// schema_hints/metadata may be absent. When present, shells must keep
// tools:{} so they emit no parameter overlays. Production embed loading
// always passes nil/empty for metadata.
func TestEmbeddedMetadataHintShellsHaveEmptyTools(t *testing.T) {
	commands, err := loadParameterCommandsFromMetadata(nil, "")
	if err != nil {
		t.Fatalf("loadParameterCommandsFromMetadata(nil, \"\"): %v", err)
	}
	if got := len(commands); got != 0 {
		t.Fatalf("nil metadata parameter overlays = %d, want 0", got)
	}

	const metadataDir = "schema_hints/metadata"
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("read %s: %v", metadataDir, err)
	}
	if len(entries) == 0 {
		return
	}

	metadataFS := os.DirFS(metadataDir)
	files, err := fs.Glob(metadataFS, "*.json")
	if err != nil {
		t.Fatalf("list metadata shells: %v", err)
	}
	sort.Strings(files)

	nonEmpty := make([]string, 0)
	for _, name := range files {
		data, err := fs.ReadFile(metadataFS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var file hintDirFile
		if err := json.Unmarshal(data, &file); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		if len(file.Tools) != 0 {
			paths := make([]string, 0, len(file.Tools))
			for canonical := range file.Tools {
				paths = append(paths, canonical)
			}
			sort.Strings(paths)
			nonEmpty = append(nonEmpty, filepath.Base(name)+"="+strings.Join(paths, ","))
		}
	}
	if len(nonEmpty) != 0 {
		t.Fatalf("metadata shells must keep tools:{} (want 0 tool rows); nonempty=%v", nonEmpty)
	}

	commands, err = loadParameterCommandsFromMetadata(metadataFS, "*.json")
	if err != nil {
		t.Fatalf("loadParameterCommandsFromMetadata: %v", err)
	}
	if got := len(commands); got != 0 {
		t.Fatalf("on-disk metadata parameter overlays = %d, want 0", got)
	}
}

// TestEmbeddedCatalogHasNoToolSchemaHintProvenance asserts the published
// Catalog no longer selects tool_schema_hint as a provenance winner or
// retained candidate after RegisterSchemaHints → ParamDecl migration.
func TestEmbeddedCatalogHasNoToolSchemaHintProvenance(t *testing.T) {
	if !embeddedSchemaCatalogAvailable() {
		t.Fatal("embedded schema catalog unavailable")
	}
	loaded := embeddedSchemaCatalog()
	hits := make([]string, 0)
	for _, product := range loaded.Registry.Products {
		for _, tool := range product.Tools {
			hits = append(hits, toolSchemaHintProvenanceHits(tool.Identity.CanonicalPath, tool.FieldProvenance)...)
			for _, param := range tool.Parameters {
				prefix := tool.Identity.CanonicalPath + "." + param.Name
				hits = append(hits, toolSchemaHintProvenanceHits(prefix, param.FieldProvenance)...)
			}
		}
	}
	if total := len(hits); total != 0 {
		sample := hits
		if total > 20 {
			sample = hits[:20]
		}
		t.Fatalf("catalog field_provenance still references tool_schema_hint (count=%d); sample=%v", total, sample)
	}
}

func toolSchemaHintProvenanceHits(prefix string, provenance map[string]FieldProvenance) []string {
	if len(provenance) == 0 {
		return nil
	}
	hits := make([]string, 0)
	fields := make([]string, 0, len(provenance))
	for field := range provenance {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		prov := provenance[field]
		if prov.Source == runtimeSchemaPrecedenceToolHint {
			hits = append(hits, prefix+"."+field+"#winner")
		}
		for i, candidate := range prov.Candidates {
			if candidate.Source == runtimeSchemaPrecedenceToolHint {
				hits = append(hits, prefix+"."+field+"#candidate["+strconv.Itoa(i)+"]")
			}
		}
		for i, candidate := range prov.OverriddenCandidates {
			if candidate.Source == runtimeSchemaPrecedenceToolHint {
				hits = append(hits, prefix+"."+field+"#overridden["+strconv.Itoa(i)+"]")
			}
		}
	}
	return hits
}
