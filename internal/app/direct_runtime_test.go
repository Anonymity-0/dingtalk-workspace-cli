package app

import (
	"strings"
	"testing"
)

// mcpdev is a helper-only product: it is absent from the generated static
// server table and from discovery, so the source pin is its only default
// resolution path. These tests guard that pin.

func TestMCPDevMCPEndpointFollowsGatewayBaseURL(t *testing.T) {
	t.Setenv("DINGTALK_MCPDEV_MCP_URL", "")

	got := mcpdevMCPEndpoint()
	if want := defaultPATGatewayBaseURL() + mcpdevServerPath; got != want {
		t.Fatalf("mcpdevMCPEndpoint() = %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, "https://") {
		t.Fatalf("mcpdevMCPEndpoint() = %q, want an https gateway URL", got)
	}
}

func TestDirectRuntimeEndpointResolvesMCPDev(t *testing.T) {
	t.Setenv("DINGTALK_MCPDEV_MCP_URL", "")

	for _, productID := range []string{mcpdevProductID, "  " + mcpdevProductID + "  "} {
		got, ok := directRuntimeEndpoint(productID, "mcp_server_url_get")
		if !ok {
			t.Fatalf("directRuntimeEndpoint(%q) ok = false, want true", productID)
		}
		if want := mcpdevMCPEndpoint(); got != want {
			t.Fatalf("directRuntimeEndpoint(%q) = %q, want %q", productID, got, want)
		}
	}
}

func TestMCPDevEndpointEnvOverrideWins(t *testing.T) {
	const override = "https://mcp-gw.example.test/server/custom"
	t.Setenv("DINGTALK_MCPDEV_MCP_URL", override)

	got, ok := directRuntimeEndpoint(mcpdevProductID, "mcp_server_url_get")
	if !ok || got != override {
		t.Fatalf("directRuntimeEndpoint(%q) = %q %v, want %q true", mcpdevProductID, got, ok, override)
	}
}

func TestDirectRuntimeProductIDsIncludeMCPDev(t *testing.T) {
	if !DirectRuntimeProductIDs()[mcpdevProductID] {
		t.Fatal("DirectRuntimeProductIDs() is missing mcpdev")
	}
}
