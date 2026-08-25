---
category: Changed
---

- **Download host trust policy** — retires the static DingTalk/OSS download
  host allowlist from both the shared local download path (`drive +download`,
  `drive +version-download`, doc/minutes artifact downloads) and the chat
  message-resource path (`chat +messages-resource-download`,
  `--download-resources`). Download URLs still require an HTTPS domain (IP
  literals and userinfo URLs stay rejected), and non-default HTTPS ports are
  now accepted because dedicated-deployment storage domains legitimately
  serve on them; the port is not a trust signal and the dial-time SSRF policy
  is port-agnostic. SSRF
  protection now happens at dial time through a shared public-IP policy that
  refuses hosts resolving to private, loopback, link-local, or otherwise
  non-public addresses. Dedicated-deployment storage domains now download
  through the same code path as public DingTalk/OSS hosts.
- **Upload host trust unchanged** — upload target URLs (`drive +upload`,
  minutes audio upload) keep the pre-existing public DingTalk/OSS trusted
  host requirement through a dedicated upload validator, so removing the
  download allowlist does not widen where local file bytes can be sent.
  Download credential headers are issued together with the download URL by
  the same authenticated service response and follow it as-is on the first
  request; redirects leaving the original host still strip them. The
  dial-time non-public IP policy also covers additional IANA
  special-purpose ranges (`0.0.0.0/8`, `192.88.99.0/24`, `100::/64`,
  `2002::/16`, `3fff::/20`, `5f00::/16`) and IPv4-embedded IPv6 transition
  ranges: NAT64 well-known prefix answers (`64:ff9b::/96`) are re-validated
  against their embedded IPv4 address so DNS64 keeps working while embedded
  internal addresses are refused, and NAT64 local-use (`64:ff9b:1::/48`) and
  Teredo (`2001::/32`) answers are refused outright.
