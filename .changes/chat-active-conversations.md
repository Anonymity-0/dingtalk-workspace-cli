---
category: Added
---

- **Chat active conversations** — adds `dws chat +active-conversations --start <time>` to auto-page cross-conversation messages and return deduplicated conversation summaries with stable name fields, latest-message time, and completeness metadata. Pagination waits 200ms between pages by default; later-page failures preserve completed summaries and a continuation cursor as `partial_failure` (exit 7). Resume with the original time window, page size, and profile, then merge batches by conversation ID.
