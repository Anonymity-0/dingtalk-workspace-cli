#!/bin/sh
set -eu

# Regenerate deterministic downstream release assets from the reviewed
# CommandRegistry into a temporary directory and compare them with the
# committed files.

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$ROOT"
. "$ROOT/scripts/policy/policy-runtime.sh"
policy_prepare_runtime "$ROOT"

tmp="$(mktemp -d)"
exec_tmp="$(policy_runtime_mktemp_dir dws-generated-drift)"
metadata_generator="$exec_tmp/schema-agent-metadata"
catalog_generator="$exec_tmp/schema-catalog"
param_aliases_generator="$exec_tmp/param-aliases"
trap 'rm -rf "$tmp" "$exec_tmp"' EXIT HUP INT TERM

go build -o "$metadata_generator" ./internal/generator/cmd_schema_agent_metadata
go build -a -o "$catalog_generator" ./internal/generator/cmd_schema_catalog
go build -o "$param_aliases_generator" ./internal/generator/cmd_param_aliases

# CommandRegistry is a reviewed input, never a generated artifact. Keep an
# independent byte-for-byte guard around the ordinary downstream generators.
registry_guard="$tmp/schema_command_registry"
cp -R internal/cli/schema_command_registry "$registry_guard"
# The parameter concept dictionary + its schema are reviewed inputs too.
concepts_guard="$tmp/param_concepts.json"
concepts_schema_guard="$tmp/param_concepts.schema.json"
cp internal/cli/param_concepts.json "$concepts_guard"
cp internal/cli/param_concepts.schema.json "$concepts_schema_guard"
# Guard human-authored selection (required) and metadata (optional).
# schema_hints/metadata may be absent, an empty directory, or hold only
# reserved product shells with tools: {} / missing tools — none of those
# alone is drift. When the directory is present, keep a byte guard so
# generation cannot rewrite those shells; never fail solely because the
# directory is missing or every product shell has empty tools.
metadata_dir="internal/cli/schema_hints/metadata"
metadata_guard=""
if [ -d "$metadata_dir" ]; then
	metadata_guard="$tmp/metadata-hints"
	cp -R "$metadata_dir" "$metadata_guard"
fi
selection_guard="$tmp/selection-hints"
cp -R internal/cli/schema_hints/selection "$selection_guard"

metadata_tmp="$tmp/metadata"
audit_tmp="$tmp/audit.json"
catalog_tmp="$tmp/schema_catalog"
catalog_tmp_second="$tmp/schema_catalog-second"
param_aliases_tmp="$tmp/param_aliases_generated.go"
param_aliases_tmp_second="$tmp/param_aliases_generated-second.go"

"$metadata_generator" \
  -root . \
  -registry internal/cli/schema_command_registry \
  -output-dir "$metadata_tmp" \
  -audit-output "$audit_tmp"

if ! diff -qr internal/cli/schema_agent_metadata "$metadata_tmp" >/dev/null; then
	printf '%s\n' 'generated drift: internal/cli/schema_agent_metadata is stale' >&2
	printf '%s\n' 'run: make generate-schema' >&2
	diff -ru internal/cli/schema_agent_metadata "$metadata_tmp" || true
	exit 1
fi

if ! cmp -s internal/cli/schema_agent_metadata_audit.json "$audit_tmp"; then
	printf '%s\n' 'generated drift: internal/cli/schema_agent_metadata_audit.json is stale' >&2
	printf '%s\n' 'run: make generate-schema' >&2
	diff -u internal/cli/schema_agent_metadata_audit.json "$audit_tmp" || true
	exit 1
fi

"$catalog_generator" \
	-root . \
	-output "$catalog_tmp"

"$catalog_generator" \
	-root . \
	-output "$catalog_tmp_second"

"$param_aliases_generator" \
	-root . \
	-output "$param_aliases_tmp"

"$param_aliases_generator" \
	-root . \
	-output "$param_aliases_tmp_second"

if ! diff -qr internal/cli/schema_command_registry "$registry_guard" >/dev/null; then
	printf '%s\n' 'generation modified reviewed input internal/cli/schema_command_registry/' >&2
	exit 1
fi

if ! cmp -s internal/cli/param_concepts.json "$concepts_guard"; then
	printf '%s\n' 'generation modified reviewed input internal/cli/param_concepts.json' >&2
	exit 1
fi

if ! cmp -s internal/cli/param_concepts.schema.json "$concepts_schema_guard"; then
	printf '%s\n' 'generation modified reviewed input internal/cli/param_concepts.schema.json' >&2
	exit 1
fi

# Optional metadata: presence/absence and empty tools:{} shells are all OK.
# Fail only if generation mutates, removes, or invents the directory tree.
if [ -n "$metadata_guard" ]; then
	if [ ! -d "$metadata_dir" ]; then
		printf '%s\n' 'generation removed reviewed input internal/cli/schema_hints/metadata' >&2
		exit 1
	fi
	if ! diff -qr "$metadata_dir" "$metadata_guard" >/dev/null; then
		printf '%s\n' 'generation modified reviewed input internal/cli/schema_hints/metadata' >&2
		exit 1
	fi
elif [ -d "$metadata_dir" ]; then
	printf '%s\n' 'generation created reviewed input internal/cli/schema_hints/metadata' >&2
	exit 1
fi

if ! diff -qr internal/cli/schema_hints/selection "$selection_guard" >/dev/null; then
	printf '%s\n' 'generation modified reviewed input internal/cli/schema_hints/selection' >&2
	exit 1
fi

if ! diff -qr "$catalog_tmp" "$catalog_tmp_second" >/dev/null; then
	printf '%s\n' 'generated drift: consecutive Catalog generations are not byte-identical' >&2
	diff -ru "$catalog_tmp" "$catalog_tmp_second" || true
	exit 1
fi

if ! diff -qr internal/cli/schema_catalog "$catalog_tmp" >/dev/null; then
	printf '%s\n' 'generated drift: internal/cli/schema_catalog is stale' >&2
	printf '%s\n' 'run: make generate-schema' >&2
	diff -ru internal/cli/schema_catalog "$catalog_tmp" || true
	exit 1
fi

if ! cmp -s "$param_aliases_tmp" "$param_aliases_tmp_second"; then
	printf '%s\n' 'generated drift: consecutive parameter-alias generations are not byte-identical' >&2
	diff -u "$param_aliases_tmp" "$param_aliases_tmp_second" || true
	exit 1
fi

if ! cmp -s internal/cli/param_aliases_generated.go "$param_aliases_tmp"; then
	printf '%s\n' 'generated drift: internal/cli/param_aliases_generated.go is stale' >&2
	printf '%s\n' 'run: make generate-schema' >&2
	diff -u internal/cli/param_aliases_generated.go "$param_aliases_tmp" || true
	exit 1
fi

printf 'generated drift check: ok\n'
