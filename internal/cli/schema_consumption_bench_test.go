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
	"crypto/sha256"
	"io/fs"
	"strings"
	"testing"
)

// These benchmarks attribute the Schema consumption cost that ResolveMeta pays
// on its first hit. Both production entry points are sync.Once memoized, so the
// benchmarks call the underlying work directly — measuring the exported wrappers
// would time one real run followed by N no-ops.
//
// They establish the RFC §M0 deliverable-11 baseline for embedded parse and
// lookup construction, which §M4's per-PR budget needs in order to mean anything.

// BenchmarkAssembleEmbeddedSchemaCatalog measures decoding the embedded release
// snapshot: the global envelope plus every per-product tools shard.
func BenchmarkAssembleEmbeddedSchemaCatalog(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		loaded, err := assembleEmbeddedSchemaCatalog()
		if err != nil {
			b.Fatal(err)
		}
		if len(loaded.Snapshot.Tools) == 0 {
			b.Fatal("decoded an empty catalog")
		}
	}
}

// BenchmarkBuildMetaByCLIPath measures turning the decoded snapshot into the
// ResolveMeta lookup map, isolated from the decode above.
func BenchmarkBuildMetaByCLIPath(b *testing.B) {
	loaded := embeddedSchemaCatalog()
	if len(loaded.Snapshot.Tools) == 0 {
		b.Skip("embedded catalog unavailable")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if len(buildMetaByCLIPath(loaded)) == 0 {
			b.Fatal("built an empty lookup")
		}
	}
}

// BenchmarkResolveMetaSteadyState measures a warm lookup, i.e. what every
// --help Safety annotation costs once the map exists.
func BenchmarkResolveMetaSteadyState(b *testing.B) {
	if _, ok := ResolveMeta("dev app delete"); !ok {
		b.Skip("embedded catalog unavailable")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, ok := ResolveMeta("dev app delete"); !ok {
			b.Fatal("warm lookup missed")
		}
	}
}

// The benchmarks below attribute the cost *inside* assembleEmbeddedSchemaCatalog,
// which the whole-path benchmark shows costs ~1.5s and ~817 MB. Optimizing the
// wrong stage is the main risk here, so each stage is measured separately.

// BenchmarkCatalogStageDecodeShards measures only the JSON decode of the
// envelope plus the 27 per-product shards into the untyped snapshot maps.
func BenchmarkCatalogStageDecodeShards(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		snapshot, err := assembleSchemaCatalogSnapshot(
			embeddedSchemaCatalogEnvelopeJSON, embeddedSchemaCatalogTools, "schema_catalog/tools")
		if err != nil {
			b.Fatal(err)
		}
		if len(snapshot.Tools) == 0 {
			b.Fatal("decoded an empty snapshot")
		}
	}
}

// BenchmarkCatalogStageSourceHash measures re-hashing the decoded snapshot,
// which the loader does on every process start to verify source_hash.
func BenchmarkCatalogStageSourceHash(b *testing.B) {
	snapshot, err := assembleSchemaCatalogSnapshot(
		embeddedSchemaCatalogEnvelopeJSON, embeddedSchemaCatalogTools, "schema_catalog/tools")
	if err != nil {
		b.Skip("embedded catalog unavailable")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if schemaCatalogSnapshotHash(snapshot) == "" {
			b.Fatal("empty hash")
		}
	}
}

// BenchmarkCatalogStageTypedRegistry measures converting the untyped snapshot
// maps into the typed registry and index.
func BenchmarkCatalogStageTypedRegistry(b *testing.B) {
	snapshot, err := assembleSchemaCatalogSnapshot(
		embeddedSchemaCatalogEnvelopeJSON, embeddedSchemaCatalogTools, "schema_catalog/tools")
	if err != nil {
		b.Skip("embedded catalog unavailable")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		registry, _, err := schemaRegistryFromSnapshot(snapshot)
		if err != nil {
			b.Fatal(err)
		}
		if len(registry.Products) == 0 {
			b.Fatal("empty registry")
		}
	}
}

// BenchmarkCatalogStageValidations measures the three whole-registry validation
// passes the loader runs after conversion.
func BenchmarkCatalogStageValidations(b *testing.B) {
	loaded := embeddedSchemaCatalog()
	if len(loaded.Registry.Products) == 0 {
		b.Skip("embedded catalog unavailable")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := loadCatalogValidateInterfaces(loaded.Registry); err != nil {
			b.Fatal(err)
		}
		if err := loadCatalogValidateProvenance(loaded.Registry); err != nil {
			b.Fatal(err)
		}
		if err := loadCatalogValidateAgentMetadata(loaded.Registry); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCatalogStageRawByteHash measures hashing the embedded bytes directly,
// as an alternative to schemaCatalogSnapshotHash, which re-marshals the decoded
// maps. It is the cost a raw-bytes integrity check would pay instead.
func BenchmarkCatalogStageRawByteHash(b *testing.B) {
	entries, err := fs.ReadDir(embeddedSchemaCatalogTools, "schema_catalog/tools")
	if err != nil {
		b.Skip("embedded catalog unavailable")
	}
	shards := make([][]byte, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, readErr := fs.ReadFile(embeddedSchemaCatalogTools, "schema_catalog/tools/"+entry.Name())
		if readErr != nil {
			b.Skip("embedded shard unavailable")
		}
		shards = append(shards, data)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sum := sha256.New()
		sum.Write(embeddedSchemaCatalogEnvelopeJSON)
		for _, shard := range shards {
			sum.Write(shard)
		}
		if len(sum.Sum(nil)) != sha256.Size {
			b.Fatal("short digest")
		}
	}
}
