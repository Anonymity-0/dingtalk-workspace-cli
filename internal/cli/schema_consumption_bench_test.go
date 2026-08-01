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
// Cold-start attribution (Apple M3 Pro, benchtime=3x, 2026-08-01):
//
// Before (map[string]any + per-tool remarshal + typed round-trip):
//
//	BenchmarkAssembleEmbeddedSchemaCatalog     ~1091ms  732MB  9.6M allocs
//	BenchmarkCatalogStageDecodeShards           ~154ms  145MB  1.4M allocs
//	BenchmarkCatalogStageTypedRegistry          ~892ms  582MB  8.1M allocs
//
// After (JSON → schemaToolWire → ToolSpec; round-trip off in production):
//
//	BenchmarkAssembleEmbeddedSchemaCatalog      ~294ms  174MB  1.2M allocs  (~3.7× / ~4.2× / ~7.8×)
//	BenchmarkCatalogStageDecodeTyped            ~173ms  117MB  0.55M allocs
//	BenchmarkCatalogStageTypedRegistryFromWire  ~120ms   40MB  0.45M allocs
//	BenchmarkResolveMetaSteadyState             ~222ns    0B   0 allocs
//
// Package tests still enable validateSnapshotTypedRoundTrip via
// schema_snapshot_roundtrip_test.go; benches force it off so they measure the
// production path.

func withProductionSchemaRoundTrip(b *testing.B) func() {
	b.Helper()
	prev := validateSchemaSnapshotTypedRoundTrip
	validateSchemaSnapshotTypedRoundTrip = false
	return func() { validateSchemaSnapshotTypedRoundTrip = prev }
}

// BenchmarkAssembleEmbeddedSchemaCatalog measures decoding the embedded release
// snapshot: the global envelope plus every per-product tools shard, then typed
// registry construction (production cold-start path).
func BenchmarkAssembleEmbeddedSchemaCatalog(b *testing.B) {
	restore := withProductionSchemaRoundTrip(b)
	defer restore()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		loaded, err := assembleEmbeddedSchemaCatalog()
		if err != nil {
			b.Fatal(err)
		}
		if len(loaded.Registry.Products) == 0 {
			b.Fatal("decoded an empty catalog")
		}
	}
}

// BenchmarkSchemaRegistryFromSnapshot measures the map-based loader used by
// generation/delivery decodeSchemaCatalogSnapshot (not the embed cold path).
func BenchmarkSchemaRegistryFromSnapshot(b *testing.B) {
	restore := withProductionSchemaRoundTrip(b)
	defer restore()
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

// BenchmarkBuildMetaByCLIPath measures turning the decoded snapshot into the
// ResolveMeta lookup map, isolated from the decode above.
func BenchmarkBuildMetaByCLIPath(b *testing.B) {
	loaded := embeddedSchemaCatalog()
	if len(loaded.Registry.Products) == 0 {
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

// The benchmarks below attribute the cost *inside* assembleEmbeddedSchemaCatalog.

// BenchmarkCatalogStageDecodeShards measures only the untyped JSON decode of the
// envelope plus the per-product shards into map[string]any (legacy/test path).
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

// BenchmarkCatalogStageDecodeTyped measures production shard decode: JSON bytes
// → schemaCatalogWire + map[canonical]schemaToolWire with no map[string]any.
func BenchmarkCatalogStageDecodeTyped(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		envelope, tools, err := assembleTypedSchemaCatalog(
			embeddedSchemaCatalogEnvelopeJSON, embeddedSchemaCatalogTools, "schema_catalog/tools")
		if err != nil {
			b.Fatal(err)
		}
		if envelope.Catalog.Kind == "" || len(tools) == 0 {
			b.Fatal("decoded an empty typed snapshot")
		}
	}
}

// BenchmarkCatalogStageSourceHash measures re-hashing the decoded snapshot,
// which production no longer does on cold start (drift CI covers it).
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

// BenchmarkCatalogStageTypedRegistry measures converting untyped snapshot maps
// into the typed registry (map path, round-trip off).
func BenchmarkCatalogStageTypedRegistry(b *testing.B) {
	restore := withProductionSchemaRoundTrip(b)
	defer restore()
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

// BenchmarkCatalogStageTypedRegistryFromWire measures wire→SchemaRegistry with
// no map[string]any intermediate (production convert stage).
func BenchmarkCatalogStageTypedRegistryFromWire(b *testing.B) {
	envelope, tools, err := assembleTypedSchemaCatalog(
		embeddedSchemaCatalogEnvelopeJSON, embeddedSchemaCatalogTools, "schema_catalog/tools")
	if err != nil {
		b.Skip("embedded catalog unavailable")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		registry, _, err := schemaRegistryFromTyped(envelope.Catalog, tools)
		if err != nil {
			b.Fatal(err)
		}
		if len(registry.Products) == 0 {
			b.Fatal("empty registry")
		}
	}
}

// BenchmarkCatalogStageValidations measures the whole-registry validation
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
