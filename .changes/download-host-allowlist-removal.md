---
category: Changed
---

- **Download host trust policy** — retires the static DingTalk/OSS download
  host allowlist from both the shared local download path (`drive +download`,
  `drive +version-download`, doc/minutes artifact downloads) and the chat
  message-resource path (`chat +messages-resource-download`,
  `--download-resources`). Download URLs still require an HTTPS domain on the
  default port (IP literals and userinfo URLs stay rejected), and SSRF
  protection now happens at dial time through a shared public-IP policy that
  refuses hosts resolving to private, loopback, link-local, or otherwise
  non-public addresses. Dedicated-deployment storage domains now download
  through the same code path as public DingTalk/OSS hosts.
