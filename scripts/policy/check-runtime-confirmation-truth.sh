#!/bin/sh
set -eu

# Ensure catalog confirmation=user_required exactly matches executable truth:
# typed corecmd Contract SafetySpec declarations plus any residual migration-only
# metadata runtime_gate != none entries.
#
# schema_hints/metadata is optional: the directory may be absent, contain no
# JSON files, or hold only empty product shells (tools: {} / missing tools).
# Empty shells contribute zero gated keys; Contract SafetySpec remains the
# production truth source.

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$ROOT"

metadata_dir="internal/cli/schema_hints/metadata"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

# The release Catalog is committed as a per-product split; reassemble it into
# the single-document shape these jq queries consume.
catalog="$tmp/catalog-combined.json"
scripts/policy/with-catalog.sh >"$catalog"

: >"$tmp/metadata_files"
: >"$tmp/metadata_gated"
if [ -d "$metadata_dir" ]; then
	# Collect JSON paths without relying on nullglob (POSIX sh).
	find "$metadata_dir" -maxdepth 1 -type f -name '*.json' | sort >"$tmp/metadata_files"
fi

if [ -s "$tmp/metadata_files" ]; then
	# shellcheck disable=SC2046
	# (.tools // {}) treats missing/null tools as an empty object so reserved
	# product shells without tool rows do not fail the gate.
	jq -r '
	  (.tools // {})
	  | to_entries[]
	  | select((.value.runtime_gate // "none") != "none")
	  | .key
	' $(cat "$tmp/metadata_files") >"$tmp/metadata_gated"
fi

jq -r '
  .tools
  | to_entries[]
  | select(.value.confirmation == "user_required")
  | select(.value.field_provenance.confirmation.source == "corecmd.contract")
  | .key
' "$catalog" >"$tmp/contract_gated"

cat "$tmp/metadata_gated" "$tmp/contract_gated" | sort -u >"$tmp/truth_gated"

jq -r '
  .tools
  | to_entries[]
  | select(.value.confirmation == "user_required")
  | .key
' "$catalog" | sort >"$tmp/catalog_required"

if ! cmp -s "$tmp/truth_gated" "$tmp/catalog_required"; then
	printf '%s\n' 'catalog confirmation=user_required differs from Contract SafetySpec + residual metadata runtime_gate truth' >&2
	printf '%s\n' 'declare corecmd Safety.Confirmation (or residual migration-only metadata runtime_gate), then regenerate schema' >&2
	diff -u "$tmp/truth_gated" "$tmp/catalog_required" || true
	exit 1
fi

: >"$tmp/gate_problems"
if [ -s "$tmp/metadata_files" ]; then
	# shellcheck disable=SC2046
	jq -s '
	  reduce .[] as $file ({};
	    . * (
	      ($file.tools // {})
	      | to_entries
	      | map({key: .key, value: (.value.runtime_gate // "none")})
	      | from_entries
	    )
	  )
	' $(cat "$tmp/metadata_files") >"$tmp/gates.json"

	jq -r --slurpfile catalog "$catalog" '
	  . as $gate_map |
	  ($catalog[0].tools) as $tools |
	  $gate_map
	  | to_entries[]
	  | .key as $canonical
	  | .value as $gate
	  | $tools[$canonical] as $tool
	  | select($tool != null)
	  | select($tool.field_provenance.confirmation.source != "corecmd.contract")
	  | select(
	      if $gate == "none" then
	        $tool.confirmation != "not_required" or $tool.risk == "high" or $tool.effect == "destructive"
	      else
	        $tool.confirmation != "user_required"
	      end
	    )
	  | "\(.key)\tgate=\(.value)\teffect=\($tool.effect // "MISSING")\trisk=\($tool.risk // "MISSING")\tconfirmation=\($tool.confirmation // "MISSING")"
	' "$tmp/gates.json" >"$tmp/gate_problems" || true
fi

if [ -s "$tmp/gate_problems" ]; then
	printf '%s\n' 'schema_hints/metadata runtime_gate disagree with catalog effect/risk/confirmation' >&2
	cat "$tmp/gate_problems" >&2
	exit 1
fi

printf '%s\n' "runtime confirmation truth ok ($(wc -l <"$tmp/truth_gated" | tr -d ' ') gated)"
