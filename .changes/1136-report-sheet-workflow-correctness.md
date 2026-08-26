---
category: Changed
---

- **Report latest lookup** — scans bounded, strictly advancing outbox pages, reconciles duplicate IDs, and reads back the uniquely newest report instead of failing on the first continuation page.
- **Sheet create-with-data result** — returns the already probed `sheetId` with a flat, declared result shape and performs bounded readback verification without repeating the sheet-list probe.
- **Sheet workflow routing** — distinguishes local analysis from Excel-to-online import, exposes template discovery and apply routes, and preserves the full data-validation tri-state contract.
- **Received-report helper** — uses bounded complete pagination, renders epoch timestamps in the Shanghai timezone, fails closed instead of returning incomplete data, and keeps midnight query windows valid.
