---
category: Added
---

- **Doc-business delegation auth** — the `drive`, `doc`, `sheet`, `wiki`, and `markdown` command groups now accept a persistent `--principal-user-id` flag. When set, the first invocation of each doc-business tool key per node within a session is gated by a `check_capability` verification on behalf of the principal; granting the capability is an out-of-band action the principal completes on the server side, and the CLI never calls `grant_capability`. A denied check surfaces the server's denial message and blocks the original call.
- **Dry-run consistency** — `checkCapability` now executes in dry-run mode as well, ensuring preview and execution behaviors are consistent.
- **Local rejection for node-less commands** — commands that lack a node identifier (e.g. search/list/create without nodeId) now return a clear client-side error (`DELEGATION_AUTH_NOT_SUPPORTED`, exit code 3) when `--principal-user-id` is set, instead of forwarding an incomplete request to the server.
