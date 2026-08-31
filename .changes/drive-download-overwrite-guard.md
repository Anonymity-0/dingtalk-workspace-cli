---
category: Changed
---

- **Drive download overwrite guard** — `dws drive download` and `dws drive download-version` now reject downloads when the target file already exists, returning a structured `INPUT_FILE_ALREADY_EXISTS` error with recovery guidance; pass `--overwrite` to proceed. Re-running the same download used to silently overwrite the existing file. Resume artifacts (`.dwspart`/`.dwspart.meta`) are not treated as conflicts.
