---
category: Changed
---

- **CLI auth apply pending page** (#1285) — the browser apply flow now lands on a
  dedicated approval-pending page that polls and auto-redirects after approval;
  duplicate apply requests are idempotent, and all local callback pages and API
  responses are served with `Cache-Control: no-store` to avoid stale state after
  a page refresh.
