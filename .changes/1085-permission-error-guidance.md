---
category: Changed
---

- **Permission error guidance and error rendering** (#1085) —
  permission-denied responses now exit with the `AUTH_PERMISSION_DENIED` code
  instead of a generic business-error rendering; document/wiki-specific errors
  (node access codes such as `forbidden.accessDenied` / `NO_PERMISSION` /
  `forbidden.no.auth`, or the role-threshold wording like
  “需要您具备 MANAGER 及以上角色”) carry apply-permission guidance
  (`dws drive permission apply-info` / `dws drive permission apply`), while
  permission failures from other products keep their product-specific or
  product-neutral suggestion instead of a misleading document-permission hint;
  member-validation failures such as
  “用户不存在/不属于当前组织” are classified as tool errors with a
  `--members`-with-`corpId` suggestion instead of a misleading
  resource-not-found error; business error output now surfaces the backend
  message with `code`/`logId` appended for traceability; and the
  `update_permission` / `remove_permission` / `update_member` /
  `remove_member` tools — whose servers return a literal `null` on successful
  no-payload writes — now render `{}` so downstream JSON consumers do not fail
  parsing `null`; other tools keep raw `null` output unchanged.
