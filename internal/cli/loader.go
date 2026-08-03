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
	"context"
	"fmt"
	"log/slog"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

// DiscoveryDegradedReason identifies why catalog discovery returned empty.
type DiscoveryDegradedReason string

const (
	DegradedUnauthenticated   DiscoveryDegradedReason = "unauthenticated"
	DegradedMarketUnreachable DiscoveryDegradedReason = "market_unreachable"
	DegradedRuntimeAllFailed  DiscoveryDegradedReason = "runtime_all_failed"
)

// DiscoveryDegraded is returned by EnvironmentLoader.Load when discovery
// fails for a diagnosable reason. Callers that need graceful degradation
// (e.g. the runtime runner) can check errors.As and fall back to an
// empty catalog; callers like the schema command can surface the hint.
type DiscoveryDegraded struct {
	Reason      DiscoveryDegradedReason
	Hint        string
	ServerCount int
}

func (e *DiscoveryDegraded) Error() string { return string(e.Reason) + ": " + e.Hint }

func degradedHint(reason DiscoveryDegradedReason, serverCount int) string {
	embedded := edition.Get().IsEmbedded
	switch reason {
	case DegradedUnauthenticated:
		if embedded {
			return "未登录，请重新认证"
		}
		return "未登录，无法发现 MCP 服务。请先执行: dws auth login"
	case DegradedMarketUnreachable:
		if embedded {
			return "无法连接 MCP 市场，请检查网络"
		}
		return "无法连接 MCP 市场，请检查网络"
	case DegradedRuntimeAllFailed:
		if embedded {
			return fmt.Sprintf("已发现 %d 个服务但连接全部失败，请稍后重试", serverCount)
		}
		return fmt.Sprintf("已发现 %d 个服务但连接全部失败；静态端点模式下请检查 internal/syncdata 生成物或稍后重试", serverCount)
	default:
		return "MCP 服务发现失败"
	}
}

func newDiscoveryDegraded(reason DiscoveryDegradedReason, serverCount int) *DiscoveryDegraded {
	return &DiscoveryDegraded{
		Reason:      reason,
		Hint:        degradedHint(reason, serverCount),
		ServerCount: serverCount,
	}
}

const (
	CatalogFixtureEnv    = "DWS_CATALOG_FIXTURE"
	CacheDirEnv          = "DWS_CACHE_DIR"
	PluginColdTimeoutEnv = "DWS_PLUGIN_COLD_TIMEOUT"
)

// ──────────────────────────────────────────────────────────────────────────
// DiscoveryCatalog types (formerly in internal/ir, now inlined as minimal stubs)
// ──────────────────────────────────────────────────────────────────────────

// DiscoveryCatalog holds the discovered MCP product surface.
type DiscoveryCatalog struct {
	Products []CanonicalProduct `json:"products"`
}

// FindProduct returns the product with the given ID.
func (c DiscoveryCatalog) FindProduct(id string) (CanonicalProduct, bool) {
	for _, product := range c.Products {
		if product.ID == id {
			return product, true
		}
	}
	return CanonicalProduct{}, false
}

// CanonicalProduct represents a single MCP product in the catalog.
type CanonicalProduct struct {
	ID          string           `json:"id"`
	DisplayName string           `json:"display_name"`
	Description string           `json:"description,omitempty"`
	ServerKey   string           `json:"server_key"`
	Endpoint    string           `json:"endpoint"`
	Source      string           `json:"source,omitempty"`
	Tools       []ToolDescriptor `json:"tools"`
}

// FindTool returns the tool with the given RPC name.
func (p CanonicalProduct) FindTool(name string) (ToolDescriptor, bool) {
	for _, tool := range p.Tools {
		if tool.RPCName == name {
			return tool, true
		}
	}
	return ToolDescriptor{}, false
}

// ToolDescriptor represents a single tool in a canonical product.
type ToolDescriptor struct {
	RPCName       string `json:"rpc_name"`
	CanonicalPath string `json:"canonical_path"`
}

// CLIFlagHint holds CLI flag alias/shorthand metadata for a tool parameter.
type CLIFlagHint struct {
	Shorthand string `json:"shorthand,omitempty"`
	Alias     string `json:"alias,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────
// DiscoveryCatalogLoader interface
// ──────────────────────────────────────────────────────────────────────────

// DiscoveryCatalogLoader loads the canonical catalog.
type DiscoveryCatalogLoader interface {
	Load(context.Context) (DiscoveryCatalog, error)
}

// StaticDiscoveryLoader returns a pre-built catalog.
type StaticDiscoveryLoader struct {
	DiscoveryCatalog DiscoveryCatalog
}

func (l StaticDiscoveryLoader) Load(_ context.Context) (DiscoveryCatalog, error) {
	return l.DiscoveryCatalog, nil
}

// DiscoveryCatalogLoaderFrom creates a DiscoveryCatalogLoader that returns a
// pre-loaded catalog and error.
func DiscoveryCatalogLoaderFrom(catalog DiscoveryCatalog, err error) DiscoveryCatalogLoader {
	return &preloadedLoader{catalog: catalog, err: err}
}

type preloadedLoader struct {
	catalog DiscoveryCatalog
	err     error
}

func (l *preloadedLoader) Load(_ context.Context) (DiscoveryCatalog, error) {
	return l.catalog, l.err
}

// ──────────────────────────────────────────────────────────────────────────
// EnvironmentLoader (static endpoint mode — no longer does live discovery)
// ──────────────────────────────────────────────────────────────────────────

// EnvironmentLoader provides catalog loading. In the post-discovery
// architecture this always returns an empty catalog; endpoint resolution
// is handled by the direct runtime path.
type EnvironmentLoader struct {
	LookupEnv              func(string) (string, bool)
	CatalogBaseURLOverride string
	AuthTokenFunc          func(context.Context) string
	LoggerFunc             func() *slog.Logger
}

// NewEnvironmentLoader creates an EnvironmentLoader with default settings.
func NewEnvironmentLoader() EnvironmentLoader {
	return EnvironmentLoader{}
}

// Load returns an empty catalog. All endpoint resolution now uses the
// direct runtime path (dynamic server registry).
func (l EnvironmentLoader) Load(_ context.Context) (DiscoveryCatalog, error) {
	return DiscoveryCatalog{}, nil
}

// Ensure unused imports are consumed.
var _ = newDiscoveryDegraded
