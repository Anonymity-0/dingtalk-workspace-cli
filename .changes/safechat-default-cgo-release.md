---
category: Fixed
---

- **SafeChat 默认构建与官方产物** — 支持平台的 CGO 构建无需额外 build tag
  即包含 SafeChat 后端，官方六平台 Release 固定使用可校验的交叉编译工具链并拒绝
  发布 CGO-disabled stub 二进制。
- 本地 `make build` / `make rebuild` 默认启用 CGO，并保留显式
  `CGO_ENABLED=0` 的 stub 构建选择。
- Linux 官方构建显式使用 glibc 2.17 链接目标，并校验 ELF 符号版本，
  防止交叉编译镜像升级隐式提高 Linux 系统要求。
