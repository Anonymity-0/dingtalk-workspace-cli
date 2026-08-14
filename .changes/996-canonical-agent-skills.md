---
category: Fixed
---

- **Canonical Agent Skills** (#996) — installs bundled DWS Skills once under `~/.agents/skills`, migrates duplicate Agent copies, and matches the upstream 76-Agent registry across Go, npm, Shell, and PowerShell. Non-universal Agents use links (Windows junctions) with safe copy fallback; custom/XDG homes and OpenClaw legacy aliases are preserved. Upgrades now back up and restore Skills safely across external volumes by staging, lexically copying links (including dangling links), verifying contents, and deleting the source only after publication succeeds. Atomic no-replace publication and identity-checked quarantine rollback preserve concurrent user changes instead of overwriting or recursively deleting them. Standalone installers verify every downloaded release asset against `checksums.txt`, and the npm engine declaration now reflects the actual Node 16.7+ API floor.
