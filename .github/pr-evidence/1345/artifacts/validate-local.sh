#!/bin/sh
set -eu

base_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
files="a2ui-create.json a2ui-step-2.json a2ui-finish.json"

for name in $files; do
  file="$base_dir/$name"
  jq -e 'type == "array" and length > 0' "$file" >/dev/null
  jq -c 'map(tojson) | map(fromjson)' "$file" | jq -e --slurpfile expected "$file" '. == $expected[0]' >/dev/null
done

surface_ids=$(jq -r '.. | objects | .surfaceId? // empty' "$base_dir"/*.json | sort -u)
[ "$surface_ids" = "example-task-progress" ]

jq -e '
  any(.[]; has("createSurface")) and
  any(.[]; has("updateComponents"))
' "$base_dir/a2ui-create.json" >/dev/null

jq -e '
  [.[].updateDataModel.path] == ["/execution/title", "/answer/displayText"]
' "$base_dir/a2ui-step-2.json" >/dev/null

jq -e '
  any(.[]; .updateDataModel.path? == "/status" and .updateDataModel.value == "finished") and
  any(.[]; .updateDataModel.path? == "/execution/done" and .updateDataModel.value == true) and
  any(.[]; .updateDataModel.path? == "/answer/done" and .updateDataModel.value == true)
' "$base_dir/a2ui-finish.json" >/dev/null

printf '%s\n' "本地校验通过：JSON、字符串数组往返、surfaceId 和阶段关键字段均一致。"
