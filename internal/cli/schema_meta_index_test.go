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
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

const schemaLazyMetaIndexChildEnv = "DWS_SCHEMA_LAZY_META_INDEX_CHILD"

func TestEmbeddedSchemaMetaIndexMatchesCatalog(t *testing.T) {
	index, err := DecodeSchemaMetaIndexJSON(embeddedSchemaMetaIndexJSON)
	if err != nil {
		t.Fatalf("DecodeSchemaMetaIndexJSON() error = %v", err)
	}
	loaded := embeddedSchemaCatalog()
	if len(loaded.Registry.Products) == 0 {
		t.Fatal("embedded catalog unavailable")
	}
	if err := ValidateSchemaMetaIndexAgainstCatalog(index, loaded.Registry); err != nil {
		t.Fatalf("ValidateSchemaMetaIndexAgainstCatalog() error = %v", err)
	}
	if index.SourceHash != loaded.Snapshot.SourceHash {
		t.Fatalf("source_hash index=%q catalog=%q", index.SourceHash, loaded.Snapshot.SourceHash)
	}
	if got := len(index.Entries); got != len(loaded.Index.CanonicalPaths()) {
		t.Fatalf("meta index entries = %d, catalog tools = %d", got, len(loaded.Index.CanonicalPaths()))
	}
}

func TestResolveMetaDoesNotLoadFullCatalog(t *testing.T) {
	if os.Getenv(schemaLazyMetaIndexChildEnv) == "1" {
		if counts := RuntimeSchemaMetadataLoadCounts(); counts != (SchemaMetadataLoadCounts{}) {
			t.Fatalf("package init loaded Schema metadata: %#v", counts)
		}
		if _, ok := ResolveMeta("dev app delete"); !ok {
			t.Fatal(`ResolveMeta("dev app delete") ok=false`)
		}
		counts := RuntimeSchemaMetadataLoadCounts()
		if counts.MetaIndex != 1 {
			t.Fatalf("MetaIndex load count = %d, want 1", counts.MetaIndex)
		}
		if counts.Catalog != 0 {
			t.Fatalf("Catalog load count = %d, want 0 after ResolveMeta", counts.Catalog)
		}
		if err := embeddedSchemaMetaIndexError(); err != nil {
			t.Fatalf("embeddedSchemaMetaIndexError() = %v", err)
		}
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestResolveMetaDoesNotLoadFullCatalog$", "-test.count=1")
	command.Env = append(os.Environ(), schemaLazyMetaIndexChildEnv+"=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("meta index lazy child failed: %v\n%s", err, strings.TrimSpace(string(output)))
	}
}

func TestEmbeddedSchemaMetaIndexLoadsOnlyOnce(t *testing.T) {
	const childEnv = "DWS_SCHEMA_LAZY_META_INDEX_ONCE_CHILD"
	if os.Getenv(childEnv) == "1" {
		var wait sync.WaitGroup
		for range 32 {
			wait.Add(1)
			go func() {
				defer wait.Done()
				_, _ = ResolveMeta("dev app delete")
			}()
		}
		wait.Wait()
		if got := RuntimeSchemaMetadataLoadCounts().MetaIndex; got != 1 {
			t.Fatalf("MetaIndex lazy load count = %d, want 1", got)
		}
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestEmbeddedSchemaMetaIndexLoadsOnlyOnce$", "-test.count=1")
	command.Env = append(os.Environ(), childEnv+"=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("meta index once child failed: %v\n%s", err, strings.TrimSpace(string(output)))
	}
}

func TestBuildSchemaMetaIndexDeterministic(t *testing.T) {
	snapshot, err := assembleSchemaCatalogSnapshot(
		embeddedSchemaCatalogEnvelopeJSON, embeddedSchemaCatalogTools, "schema_catalog/tools")
	if err != nil {
		t.Skip("embedded catalog unavailable")
	}
	first, err := BuildSchemaMetaIndex(snapshot)
	if err != nil {
		t.Fatalf("BuildSchemaMetaIndex() error = %v", err)
	}
	second, err := BuildSchemaMetaIndex(snapshot)
	if err != nil {
		t.Fatalf("second BuildSchemaMetaIndex() error = %v", err)
	}
	left, err := EncodeSchemaMetaIndex(first)
	if err != nil {
		t.Fatal(err)
	}
	right, err := EncodeSchemaMetaIndex(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(left) != string(right) {
		t.Fatal("BuildSchemaMetaIndex is not deterministic")
	}
	if err := ValidateSchemaMetaIndexAgainstSnapshot(first, snapshot); err != nil {
		t.Fatalf("ValidateSchemaMetaIndexAgainstSnapshot() error = %v", err)
	}
}
