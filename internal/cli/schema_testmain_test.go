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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

// packageCLIAssembledDelivery holds the ResolveSchemaBuild dump installed by
// TestMain for package-cli tests (cannot import app).
var packageCLIAssembledDelivery *loadedSchemaCatalog

// TestMain assembles a fresh Catalog through cmd_schema_catalog (ResolveSchemaBuild)
// and installs it as the package-cli delivery source. Package cli cannot import
// internal/app (cycle); the generator subprocess owns the app root factory.
// Production binaries never use this path — only RegisterSchemaSourceRoot from app.
func TestMain(m *testing.M) {
	if err := installAssembledSchemaDeliveryForPackageCLITests(); err != nil {
		fmt.Fprintf(os.Stderr, "schema package-cli TestMain: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func installAssembledSchemaDeliveryForPackageCLITests() error {
	repoRoot, err := repoRootFromCLIPackage()
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "dws-cli-schema-test-*")
	if err != nil {
		return err
	}
	outDir := filepath.Join(tmp, "schema_catalog")
	generator := filepath.Join(tmp, "cmd_schema_catalog")
	build := exec.Command("go", "build", "-o", generator, "./internal/generator/cmd_schema_catalog")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build cmd_schema_catalog: %w\n%s", err, out)
	}
	run := exec.Command(generator, "-root", repoRoot, "-output", outDir, "-meta-index", filepath.Join(tmp, "schema_meta_index.gob"))
	run.Dir = repoRoot
	if out, err := run.CombinedOutput(); err != nil {
		return fmt.Errorf("run cmd_schema_catalog: %w\n%s", err, out)
	}
	envelope, err := os.ReadFile(filepath.Join(outDir, "catalog.json"))
	if err != nil {
		return err
	}
	typed, tools, err := assembleTypedSchemaCatalog(envelope, os.DirFS(outDir), "tools")
	if err != nil {
		return fmt.Errorf("assemble typed catalog dump: %w", err)
	}
	loaded, err := loadTypedSchemaCatalog(typed, tools)
	if err != nil {
		return fmt.Errorf("load typed catalog dump: %w", err)
	}
	if loaded.Snapshot.Tools == nil {
		payload, err := registryToSnapshotPayload(loaded.Registry)
		if err != nil {
			return fmt.Errorf("materialize catalog maps: %w", err)
		}
		loaded.Snapshot.Catalog = payload.Catalog
		loaded.Snapshot.Tools = payload.Tools
	}
	packageCLIAssembledDelivery = &loaded
	restorePackageCLISchemaDeliveryForTest()
	return nil
}

func restorePackageCLISchemaDeliveryForTest() {
	if packageCLIAssembledDelivery == nil {
		return
	}
	loaded := *packageCLIAssembledDelivery
	schemaSourceRootFn = func() *cobra.Command {
		return &cobra.Command{Use: "dws"}
	}
	assembleDeliverySchemaCatalogFn = func(*cobra.Command) (loadedSchemaCatalog, error) {
		return loaded, nil
	}
	resetDeliverySchemaCatalogStateForTest()
	metaByCLIPathOnce = sync.Once{}
	metaByCLIPath = nil
	runtimeEmbeddedSchemaMetaIndexErr = nil
	runtimeEmbeddedSchemaMetaIndexLazyCount.Store(0)
}

func repoRootFromCLIPackage() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("repo root %q: %w", root, err)
	}
	return root, nil
}
