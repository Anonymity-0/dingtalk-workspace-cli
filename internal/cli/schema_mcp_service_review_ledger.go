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

// Reviewed MCP service disposition ledger.
//
// Migrated from the retired schema_mcp_service_review.json. Policy and tests
// read this Go source; runtime Schema assembly does not consume it.
// schema_mcp_metadata.json remains the pinned MCP tool-metadata baseline.
// Do not reintroduce a committed service-review JSON.

const (
	reviewedMCPServiceReviewVersion = 1

	// reviewedMCPServiceReviewSnapshotSourceHash must equal
	// schema_mcp_metadata.json source_hash. Update both together when the
	// pinned MCP snapshot is refreshed.
	reviewedMCPServiceReviewSnapshotSourceHash = "sha256:17251f74a4f76142457cc0e87251ecb86c1e3bda0cdf2edc02f351589716ecbc"

	mcpServiceDispositionOutOfSurface = "out_of_surface"
)

// reviewedMCPServiceDisposition is one reviewed disposition for a service that
// is absent from the pinned MCP snapshot coverage.
type reviewedMCPServiceDisposition struct {
	Status string
	Reason string
}

// reviewedMCPMissingServices is the reviewed ledger of MCP services that are
// intentionally missing from the pinned snapshot. Keys are exact service IDs;
// each entry needs a non-empty status and reason.
var reviewedMCPMissingServices = map[string]reviewedMCPServiceDisposition{
	"notify": {
		Status: mcpServiceDispositionOutOfSurface,
		Reason: "notify 不属于 reviewed CommandRegistry 的公开命令集合，且没有静态 endpoint 或受审查 RPC 快照；禁止运行时发现或推测映射",
	},
}
