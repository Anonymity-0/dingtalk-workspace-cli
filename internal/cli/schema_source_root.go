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

// SchemaSourceRuntimeAssembled is stamped on SchemaRegistry.Source for
// declare→ResolveSchemaBuild delivery.
const SchemaSourceRuntimeAssembled = "runtime-assembled"

// errSchemaSourceRootNotRegistered is returned when Catalog / ResolveMeta
// delivery is requested without a root factory.
var errSchemaSourceRootNotRegistered = fmt.Errorf(
	"schema source root factory is not registered; call RegisterSchemaSourceRoot (app.NewRootCommand or test helper)",
)

// schemaSourceRootFn builds the distribution-owned Cobra tree used to assemble
// Schema at runtime (声明即 Catalog). Without a factory, delivery fails closed
// — there is no committed Catalog/gob fallback.
var schemaSourceRootFn func() *cobra.Command

var (
	runtimeDeliverySchemaCatalogOnce      sync.Once
	runtimeDeliverySchemaCatalog          loadedSchemaCatalog
	runtimeDeliverySchemaCatalogErr       error
	runtimeDeliverySchemaCatalogLazyCount atomic.Uint64
	runtimeDeliverySchemaCatalogMapsOnce  sync.Once
	runtimeDeliverySchemaCatalogMapsErr   error
)

// RegisterSchemaSourceRoot installs the root factory used by runtime Schema
// delivery (dws schema / ResolveMeta). Production registers from internal/app.
// Passing nil clears the factory (tests only) and resets lazy delivery / Meta state.
func RegisterSchemaSourceRoot(factory func() *cobra.Command) {
	schemaSourceRootFn = factory
	resetMetaByCLIPathStateForTest()
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
	runtimeDeliverySchemaCatalogMapsOnce = sync.Once{}
	runtimeDeliverySchemaCatalogMapsErr = nil
}

// assembleSchemaCatalogFromRoot is the declare→Catalog runtime path.
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
	registry.Source = SchemaSourceRuntimeAssembled
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
	surfaceHash := resolved.RegistryHash()
	return loadedSchemaCatalog{
		Snapshot: SchemaCatalogSnapshot{
			Version:     SchemaCatalogSnapshotVersion,
			SurfaceHash: surfaceHash,
			SourceHash:  surfaceHash,
		},
		Registry: registry,
		Index:    index,
	}, nil
}

var assembleDeliverySchemaCatalogFn = assembleSchemaCatalogFromRoot

// deliverySchemaCatalog is the sole production Catalog loader. It lazily
// assembles via ResolveSchemaBuild and caches the ResolveMeta map. Without a
// factory it fails closed.
func deliverySchemaCatalog() loadedSchemaCatalog {
	runtimeDeliverySchemaCatalogOnce.Do(func() {
		runtimeDeliverySchemaCatalogLazyCount.Add(1)
		if schemaSourceRootFn == nil {
			runtimeDeliverySchemaCatalogErr = errSchemaSourceRootNotRegistered
			installDeliveryCommandMeta(loadedSchemaCatalog{}, runtimeDeliverySchemaCatalogErr)
			return
		}
		root := schemaSourceRootFn()
		if root == nil {
			runtimeDeliverySchemaCatalogErr = fmt.Errorf("schema source root factory returned nil")
			installDeliveryCommandMeta(loadedSchemaCatalog{}, runtimeDeliverySchemaCatalogErr)
			return
		}
		runtimeDeliverySchemaCatalog, runtimeDeliverySchemaCatalogErr = assembleDeliverySchemaCatalogFn(root)
		if runtimeDeliverySchemaCatalogErr != nil {
			installDeliveryCommandMeta(loadedSchemaCatalog{}, runtimeDeliverySchemaCatalogErr)
			return
		}
		installDeliveryCommandMeta(runtimeDeliverySchemaCatalog, nil)
	})
	return runtimeDeliverySchemaCatalog
}

func deliverySchemaCatalogError() error {
	_ = deliverySchemaCatalog()
	return runtimeDeliverySchemaCatalogErr
}

// materializeDeliverySchemaCatalogMaps fills Snapshot.Catalog/Tools when a
// caller still needs untyped maps.
func materializeDeliverySchemaCatalogMaps() (loadedSchemaCatalog, error) {
	_ = deliverySchemaCatalog()
	if runtimeDeliverySchemaCatalogErr != nil {
		return loadedSchemaCatalog{}, runtimeDeliverySchemaCatalogErr
	}
	runtimeDeliverySchemaCatalogMapsOnce.Do(func() {
		if runtimeDeliverySchemaCatalog.Snapshot.Tools != nil {
			return
		}
		payload, err := registryToSnapshotPayloadFn(runtimeDeliverySchemaCatalog.Registry)
		if err != nil {
			runtimeDeliverySchemaCatalogMapsErr = fmt.Errorf("materialize Schema Catalog maps: %w", err)
			return
		}
		runtimeDeliverySchemaCatalog.Snapshot.Catalog = payload.Catalog
		runtimeDeliverySchemaCatalog.Snapshot.Tools = payload.Tools
		runtimeDeliverySchemaCatalog.Snapshot.SourceHash = schemaCatalogSnapshotHash(SchemaCatalogSnapshot{
			Version:     runtimeDeliverySchemaCatalog.Snapshot.Version,
			SurfaceHash: runtimeDeliverySchemaCatalog.Snapshot.SurfaceHash,
			Catalog:     payload.Catalog,
			Tools:       payload.Tools,
		})
	})
	if runtimeDeliverySchemaCatalogMapsErr != nil {
		return loadedSchemaCatalog{}, runtimeDeliverySchemaCatalogMapsErr
	}
	return runtimeDeliverySchemaCatalog, nil
}
