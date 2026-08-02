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
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
)

// schemaSourceRootFn builds the distribution-owned Cobra tree used to assemble
// Schema at runtime (声明即 Catalog). The app package registers
// NewSchemaSourceRootCommand during init; package cli tests may leave this nil
// and fall back to the committed shard/gob fixtures.
var schemaSourceRootFn func() *cobra.Command

var (
	runtimeDeliverySchemaCatalogOnce      sync.Once
	runtimeDeliverySchemaCatalog          loadedSchemaCatalog
	runtimeDeliverySchemaCatalogErr       error
	runtimeDeliverySchemaCatalogLazyCount atomic.Uint64
)

// RegisterSchemaSourceRoot installs the root factory used by runtime Schema
// delivery (dws schema / ResolveMeta). Production registers from internal/app.
// Passing nil clears the factory (tests only).
func RegisterSchemaSourceRoot(factory func() *cobra.Command) {
	schemaSourceRootFn = factory
}

// SchemaSourceRootRegistered reports whether runtime assembly has a root factory.
func SchemaSourceRootRegistered() bool {
	return schemaSourceRootFn != nil
}

func resetDeliverySchemaCatalogStateForTest() {
	runtimeDeliverySchemaCatalogOnce = sync.Once{}
	runtimeDeliverySchemaCatalog = loadedSchemaCatalog{}
	runtimeDeliverySchemaCatalogErr = nil
	runtimeDeliverySchemaCatalogLazyCount.Store(0)
}

// assembleSchemaCatalogFromRoot is the declare→Catalog runtime path. It uses
// ResolveSchemaBuild + typed registry projection (not BuildSchemaCatalogSnapshot):
// full generate-time gates stay in CI (cmd_schema_catalog / check-schema-assembly).
func assembleSchemaCatalogFromRoot(root *cobra.Command) (loadedSchemaCatalog, error) {
	if root == nil {
		return loadedSchemaCatalog{}, fmt.Errorf("schema source root is nil")
	}
	resolved, err := ResolveSchemaBuild(root)
	if err != nil {
		return loadedSchemaCatalog{}, fmt.Errorf("resolve Schema build: %w", err)
	}
	if err := resolveValidateParameterDelivery(resolved.bound, resolved.registry); err != nil {
		return loadedSchemaCatalog{}, fmt.Errorf("validate Schema parameter binding delivery: %w", err)
	}
	registry := resolved.registry
	registry.Source = "embedded-command-catalog"
	if err := loadCatalogValidateInterfaces(registry); err != nil {
		return loadedSchemaCatalog{}, fmt.Errorf("validate Schema interface disposition: %w", err)
	}
	if err := loadCatalogValidateProvenance(registry); err != nil {
		return loadedSchemaCatalog{}, fmt.Errorf("validate Schema provenance: %w", err)
	}
	index, err := registry.Index()
	if err != nil {
		return loadedSchemaCatalog{}, fmt.Errorf("build Schema index: %w", err)
	}
	payload, err := registry.ToSnapshotPayload()
	if err != nil {
		return loadedSchemaCatalog{}, fmt.Errorf("serialize Schema registry: %w", err)
	}
	snapshot := SchemaCatalogSnapshot{
		Version:     SchemaCatalogSnapshotVersion,
		SurfaceHash: resolved.RegistryHash(),
		Catalog:     payload.Catalog,
		Tools:       payload.Tools,
	}
	snapshot.SourceHash = schemaCatalogSnapshotHash(snapshot)
	return loadedSchemaCatalog{
		Snapshot: SchemaCatalogSnapshot{
			Version:     snapshot.Version,
			SurfaceHash: snapshot.SurfaceHash,
			SourceHash:  snapshot.SourceHash,
		},
		Registry: registry,
		Index:    index,
	}, nil
}

var assembleDeliverySchemaCatalogFn = assembleSchemaCatalogFromRoot

// deliverySchemaCatalog is the production Catalog loader. When a root factory
// is registered it lazily assembles via ResolveSchemaBuild; otherwise it falls
// back to the committed schema_catalog/ embed (package-cli test residual).
func deliverySchemaCatalog() loadedSchemaCatalog {
	if schemaSourceRootFn == nil {
		return embeddedSchemaCatalog()
	}
	runtimeDeliverySchemaCatalogOnce.Do(func() {
		runtimeDeliverySchemaCatalogLazyCount.Add(1)
		root := schemaSourceRootFn()
		if root == nil {
			runtimeDeliverySchemaCatalogErr = fmt.Errorf("schema source root factory returned nil")
			return
		}
		runtimeDeliverySchemaCatalog, runtimeDeliverySchemaCatalogErr = assembleDeliverySchemaCatalogFn(root)
	})
	return runtimeDeliverySchemaCatalog
}

func deliverySchemaCatalogError() error {
	_ = deliverySchemaCatalog()
	if schemaSourceRootFn != nil {
		return runtimeDeliverySchemaCatalogErr
	}
	return embeddedSchemaCatalogError()
}
