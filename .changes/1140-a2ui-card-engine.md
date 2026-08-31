---
category: Added
---

- **Chat A2UI card engine** (#1140) — `chat message send-card` and
  `chat message update-card` accept `--card-engine streaming|a2ui` (default
  `streaming`, streaming path unchanged). With `a2ui`, `send-card` delivers
  `--content` as a JSON string array via the A2UI card tool (auto-generating
  `requestId`/`bizCardId` and a newline-joined `summary`, and resolving
  single-chat user IDs to `receiverOpenDingTalkId`), and `update-card`
  accepts flow status 1-9 mapped to the A2UI status enum.
