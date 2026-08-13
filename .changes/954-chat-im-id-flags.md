---
category: Changed
---

- **Chat IM ID flags** (#954) — standardizes chat command entry points on `--conversation-id` for conversation IDs and `--message-id` for message IDs, so help, Schema, and Agent recommendations use the same canonical flags.
- **Legacy chat flag compatibility** (#954) — keeps older chat IM ID flags such as `--group`, `--id`, `--chat`, `--open-conversation-id`, `--msg-id`, and `--open-message-id` working as compatibility aliases where applicable, while hiding migrated aliases from recommended help and Schema surfaces.
- **Chat group name flags** (#954) — moves chat commands that accept a group name to the explicit `--group-name` flag, removing the public `--group` ambiguity between group names and conversation IDs on migrated command surfaces.
