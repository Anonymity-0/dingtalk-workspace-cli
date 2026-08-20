---
category: Added
---

- **Doc public-link and historical-version reads** — `dws doc read` forwards
  the reviewed `password` (internet-public documents with password protection)
  and `historyVersion` (read content as of a listed historical version; `0`
  denotes the document's initial version) parameters on the markdown, JSONML,
  and scope read paths via `--password` / `--version`; `dws doc +fetch` gains
  `--password` and replaces its previous hard rejection of `--revision` with
  validated forwarding to `get_document_content.historyVersion`.
