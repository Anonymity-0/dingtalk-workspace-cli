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
	"sort"
	"strings"
	"testing"
)

func TestSchemaMCPServiceReviewLedgerMatchesPinnedSnapshot(t *testing.T) {
	if reviewedMCPServiceReviewVersion != 1 {
		t.Fatalf("reviewedMCPServiceReviewVersion = %d, want 1", reviewedMCPServiceReviewVersion)
	}
	pinned := loadPinnedMCPMetadata()
	if pinned.SourceHash == "" {
		t.Fatal("pinned MCP metadata source_hash is empty")
	}
	if reviewedMCPServiceReviewSnapshotSourceHash != pinned.SourceHash {
		t.Fatalf("snapshot_source_hash = %q, want pinned source_hash %q",
			reviewedMCPServiceReviewSnapshotSourceHash, pinned.SourceHash)
	}

	keys := make([]string, 0, len(reviewedMCPMissingServices))
	for serviceID, disposition := range reviewedMCPMissingServices {
		serviceID = strings.TrimSpace(serviceID)
		if serviceID == "" {
			t.Fatal("reviewedMCPMissingServices contains an empty service id")
		}
		if strings.TrimSpace(disposition.Status) == "" {
			t.Fatalf("reviewedMCPMissingServices[%q] status is empty", serviceID)
		}
		if strings.TrimSpace(disposition.Reason) == "" {
			t.Fatalf("reviewedMCPMissingServices[%q] reason is empty", serviceID)
		}
		keys = append(keys, serviceID)
	}
	sort.Strings(keys)
	if len(keys) != 1 || keys[0] != "notify" {
		t.Fatalf("reviewedMCPMissingServices keys = %v, want [notify]", keys)
	}
	notify := reviewedMCPMissingServices["notify"]
	if notify.Status != mcpServiceDispositionOutOfSurface {
		t.Fatalf("notify status = %q, want %q", notify.Status, mcpServiceDispositionOutOfSurface)
	}
}

func TestSchemaMCPServiceReviewJSONRetired(t *testing.T) {
	if _, err := os.Stat("schema_mcp_service_review.json"); err == nil {
		t.Fatal("schema_mcp_service_review.json must stay deleted; use schema_mcp_service_review_ledger.go")
	}
}
