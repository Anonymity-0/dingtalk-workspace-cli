// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import "testing"

// mustEmbeddedSchemaCatalogMaps returns the embedded catalog with Snapshot
// Catalog/Tools maps materialized. Production ResolveMeta never pays this cost.
func mustEmbeddedSchemaCatalogMaps(t *testing.T) loadedSchemaCatalog {
	t.Helper()
	loaded, err := materializeEmbeddedSchemaCatalogMaps()
	if err != nil {
		t.Fatalf("materialize embedded Schema Catalog maps: %v", err)
	}
	if len(loaded.Snapshot.Tools) == 0 {
		t.Fatal("embedded Schema Catalog tools maps are empty after materialize")
	}
	return loaded
}
