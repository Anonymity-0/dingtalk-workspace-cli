// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import "testing"

func mustDeliverySchemaCatalogMaps(t *testing.T) loadedSchemaCatalog {
	t.Helper()
	if !SchemaSourceRootRegistered() {
		t.Fatal("schema source root factory is not registered; package-cli TestMain should install assembly delivery")
	}
	loaded, err := materializeDeliverySchemaCatalogMaps()
	if err != nil {
		t.Fatalf("materialize delivery Schema Catalog maps: %v", err)
	}
	if len(loaded.Snapshot.Tools) == 0 {
		t.Fatal("delivery Schema Catalog tools maps are empty after materialize")
	}
	return loaded
}

// Compatibility aliases for existing package-cli tests.
func mustEmbeddedSchemaCatalogMaps(t *testing.T) loadedSchemaCatalog {
	return mustDeliverySchemaCatalogMaps(t)
}

func embeddedSchemaCatalog() loadedSchemaCatalog { return deliverySchemaCatalog() }
func embeddedSchemaCatalogAvailable() bool       { return deliverySchemaCatalogAvailable() }
func embeddedSchemaCatalogError() error          { return deliverySchemaCatalogError() }
func embeddedSchemaPayload(args []string) (map[string]any, error) {
	return queryDeliverySchemaPayload(args)
}
func embeddedSchemaAllPayload() (map[string]any, error)      { return deliverySchemaAllPayload() }
func embeddedSchemaOverviewPayload() (map[string]any, error) { return deliverySchemaOverviewPayload() }
func materializeEmbeddedSchemaCatalogMaps() (loadedSchemaCatalog, error) {
	return materializeDeliverySchemaCatalogMaps()
}
func resetEmbeddedSchemaCatalogStateForTest() { restorePackageCLISchemaDeliveryForTest() }
