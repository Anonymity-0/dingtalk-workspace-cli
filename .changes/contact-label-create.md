---
category: Added
---

- **Contact label create** — `dws contact label create --name <名称> --type role --parent-id <角色组ID>` creates a new role (label) under the specified parent label group; `--type group` creates a root-level label group and must omit `--parent-id` (the CLI passes parentId=-1). `--parent-id` only accepts a real group id (never 0) and is required for `--type role`. The command calls the `add_label` MCP tool and requires confirmation; use `--yes` only after explicit user confirmation.
