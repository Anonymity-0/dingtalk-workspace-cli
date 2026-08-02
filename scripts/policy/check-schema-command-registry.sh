#!/bin/sh
set -eu

# Validate the reviewed CommandRegistry as an input contract. This check is
# deliberately independent of Catalog/interface/provenance output policy so a
# malformed identity source cannot be hidden by a healthy generated snapshot.

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$ROOT"
. "$ROOT/scripts/policy/search.sh"
. "$ROOT/scripts/policy/policy-runtime.sh"
policy_prepare_runtime "$ROOT"

if [ -e internal/cli/schema_native_contracts.go ] ||
	[ -e internal/cli/schema_native_contracts_generated.go ] ||
	policy_search_go 'ApplyNativeRuntimeSchemaContracts|nativeRuntimeSchemaContracts|runtimeSchemaIdentityCandidate' internal/cli; then
	printf '%s\n' 'native Schema identity materialization must not be reintroduced' >&2
	exit 1
fi

# Legacy hint maps used to select primary CLI paths and discover helper roots.
# They are identity/navigation sources, so bringing any of them back would
# silently reintroduce a second source beside CommandRegistry.
if policy_search_go '(schemaPrimaryCLIPath|RuntimeSchemaRootHint|RegisterRuntimeSchemaRoot|PrimaryCLIPaths|RegisterSchemaProductVisibility|SchemaProductVisibilityFor|productVisibility)' \
	internal/cli; then
	printf '%s\n' 'legacy Schema hint navigation or visibility sources must not be reintroduced' >&2
	exit 1
fi

if [ ! -f internal/cli/schema_command_registry.schema.json ] ||
	! jq -e '
	  ."$schema" == "./schema_command_registry.schema.json" and
	  .version == 1
	' internal/cli/schema_command_registry/registry.json >/dev/null ||
	! ls internal/cli/schema_command_registry/products/*.json >/dev/null 2>&1; then
	printf '%s\n' 'reviewed CommandRegistry is missing its local JSON Schema contract or product shards' >&2
	exit 1
fi

# Single-track delivery (声明即 Catalog): go:generate only refreshes
# param_aliases. Catalog assembly is runtime ResolveSchemaBuild; CI proves
# determinism via check-schema-assembly.sh. Reject committed Catalog generate
# and any directive that targets the reviewed CommandRegistry input.
if ! grep -Eq '^//go:generate .*cmd_param_aliases' internal/cli/gen.go; then
	printf '%s\n' 'go generate must register the param_aliases generator' >&2
	exit 1
fi
if grep -E '^//go:generate' internal/cli/gen.go | grep -Eq 'cmd_schema_catalog'; then
	printf '%s\n' 'go generate must not register committed Catalog delivery (cmd_schema_catalog)' >&2
	exit 1
fi
if ! grep -Eq 'check-schema-assembly\.sh' Makefile; then
	printf '%s\n' 'Makefile must invoke check-schema-assembly.sh for assembly determinism' >&2
	exit 1
fi
if grep -E '^//go:generate' internal/cli/gen.go | grep -Eq 'cmd_schema_agent_metadata|schema_agent_metadata'; then
	printf '%s\n' 'go generate must not regenerate retired schema_agent_metadata/' >&2
	exit 1
fi
if policy_search_go '^//go:generate .*-(output|output-dir|audit-output)(=|[[:space:]]+)([^[:space:]]*/)?schema_command_registry\.json([[:space:]]|$)' \
	internal/cli ||
	policy_search_go '^//go:generate .*(>|>>)[[:space:]]*([^[:space:]]*/)?schema_command_registry\.json([[:space:]]|$)' \
		internal/cli; then
	printf '%s\n' 'go generate must never overwrite the reviewed CommandRegistry' >&2
	exit 1
fi

# Embedded MCP/parameter metadata is intentionally expensive and must be
# parsed only through its sync.Once accessor. Each raw loader is allowed at
# exactly two production locations: its declaration and the assignment inside
# that accessor. Any third reference is an eager initializer or an accessor
# bypass and fails this static check.
# Agent metadata JSON embed/loader is retired; production must not reopen it.
if policy_search_production_go 'go:embed schema_agent_metadata' internal/cli; then
	printf '%s\n' 'schema_agent_metadata must not be re-embedded' >&2
	exit 1
fi
if policy_search_production_go 'loadEmbeddedAgentMetadata\(' internal/cli; then
	printf '%s\n' 'retired loadEmbeddedAgentMetadata must not remain in production code' >&2
	exit 1
fi
if policy_search_production_go 'loadAgentMetadataFixtureFrom\(' internal/cli; then
	printf '%s\n' 'Agent metadata fixture loader must stay test-only' >&2
	exit 1
fi
check_schema_loader_references() {
	loader="$1"
	allowed="$2"
	references="$(policy_search_production_go "${loader}\\(" internal/cli || true)"
	count="$(printf '%s\n' "$references" | awk 'NF { count++ } END { print count + 0 }')"
	if [ "$count" -ne 2 ]; then
		printf 'Schema loader %s has %s production references, want exactly declaration + lazy accessor\n' "$loader" "$count" >&2
		printf '%s\n' "$references" >&2
		exit 1
	fi
	unexpected="$(printf '%s\n' "$references" | grep -Ev "$allowed" || true)"
	if [ -n "$unexpected" ]; then
		printf 'Schema loader %s is called outside its lazy accessor:\n%s\n' "$loader" "$unexpected" >&2
		exit 1
	fi
}

check_schema_loader_references \
	'loadPinnedMCPMetadata' \
	'^internal/cli/runtime_schema\.go:[0-9]+:(func loadPinnedMCPMetadata\(\) embeddedMCPMetadata \{|[[:space:]]*runtimePinnedMCPMetadataLazy\.metadata = loadPinnedMCPMetadata\(\))$'
check_schema_loader_references \
	'loadSchemaParameterBindingSnapshot' \
	'^internal/cli/schema_parameter_bindings\.go:[0-9]+:(func loadSchemaParameterBindingSnapshot\(\) \(schemaParameterBindingSnapshot, error\) \{|[[:space:]]*runtimeSchemaParameterBindingsLazy\.snapshot, runtimeSchemaParameterBindingsLazy\.err = loadSchemaParameterBindingSnapshot\(\))$'

# Catch the common direct eager form statically; the fresh-process tests below
# additionally catch indirect or multi-line package initializers.
if policy_search_production_go '^[[:space:]]*var .*=[[:space:]]*(runtimeAgentMetadata|runtimeMCPMetadata|runtimeSchemaParameterBindingData)\(' \
	internal/cli; then
	printf '%s\n' 'Schema metadata accessors must not be called from package-scope variable initializers' >&2
	exit 1
fi

# Root construction may register the schema command, but app production code
# must never parse or inspect generation metadata. The schema command reads the
# already embedded Catalog only when it is actually executed.
if policy_search_production_go '(loadEmbeddedAgentMetadata|loadPinnedMCPMetadata|loadSchemaParameterBindingSnapshot|runtimeAgentMetadata|runtimeMCPMetadata|runtimeSchemaParameterBindingData|EmbeddedSchemaParameterBindings)\(' \
	internal/app; then
	printf '%s\n' 'root/app production code must not access Schema generation metadata loaders or accessors' >&2
	exit 1
fi

go test ./internal/cli \
	-run '^(TestCommandRegistry.*|TestDecodeCommandRegistry.*|TestBuildEffectiveCommandRegistry.*|TestBindEffectiveCommandRegistry.*|TestRuntimeSchemaMetadataLoadsOnlyOnDemand)$' \
	-count=1

go test ./internal/app \
	-run '^TestOrdinaryRootCommandsDoNotLoadSchemaMetadata$' \
	-count=1

printf 'schema CommandRegistry check: ok (%s reviewed commands)\n' \
	"$(cat internal/cli/schema_command_registry/products/*.json | jq -s '[.[].tools[]] | length')"
