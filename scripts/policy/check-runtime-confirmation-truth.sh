#!/bin/sh
set -eu

# Ensure catalog confirmation=user_required exactly matches executable truth:
# typed corecmd Contract SafetySpec declarations.
#
# schema_hints/ is retired; residual metadata runtime_gate overlays are gone.
# Contract SafetySpec is the sole production truth source for confirmation.

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$ROOT"

if [ -e internal/cli/schema_hints ]; then
	printf '%s\n' 'retired schema_hints/ must not be present' >&2
	exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

# The release Catalog is committed as a per-product split; reassemble it into
# the single-document shape these jq queries consume.
catalog="$tmp/catalog-combined.json"
scripts/policy/with-catalog.sh >"$catalog"

jq -r '
  .tools
  | to_entries[]
  | select(.value.confirmation == "user_required")
  | select(.value.field_provenance.confirmation.source == "corecmd.contract")
  | .key
' "$catalog" | sort -u >"$tmp/truth_gated"

jq -r '
  .tools
  | to_entries[]
  | select(.value.confirmation == "user_required")
  | .key
' "$catalog" | sort >"$tmp/catalog_required"

if ! cmp -s "$tmp/truth_gated" "$tmp/catalog_required"; then
	printf '%s\n' 'catalog confirmation=user_required differs from Contract SafetySpec truth' >&2
	printf '%s\n' 'declare corecmd Safety.Confirmation, then regenerate schema' >&2
	diff -u "$tmp/truth_gated" "$tmp/catalog_required" || true
	exit 1
fi

printf '%s\n' "runtime confirmation truth ok ($(wc -l <"$tmp/truth_gated" | tr -d ' ') gated)"
