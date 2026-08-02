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
