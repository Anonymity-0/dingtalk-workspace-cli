---
category: Fixed
---

- **Canonical Agent Skills** (#996) — installs bundled DWS Skills once under `~/.agents/skills`, migrates duplicate Agent copies, and matches the upstream 76-Agent registry across Go, npm, Shell, and PowerShell. Non-universal Agents use links (Windows junctions) with safe copy fallback; custom/XDG homes and OpenClaw legacy aliases are preserved.
