#!/bin/sh
# Copyright 2026 Alibaba Group
# Licensed under the Apache License, Version 2.0
#
# One-command installer for dws dev — pre-built binary, no build tools.
# Downloads the dev binary + dingtalk-misc skill (hosts open-platform app docs) from the DingTalk-Real-AI GitHub Releases.
# Requires only curl + tar (no go / make / git).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/DingTalk-Real-AI/dingtalk-workspace-cli/main/scripts/install-devapp.sh | sh
#
# Env (all optional):
#   DEVAPP_REPO      repo holding dev releases (default: DingTalk-Real-AI/dingtalk-workspace-cli)
#   DEVAPP_VERSION   pin a release tag (default: latest release)
#   DWS_INSTALL_DIR  binary dir (default: ~/.local/bin)
#   DWS_NO_SKILLS    set 1 to skip the dev skill
set -eu

DEVAPP_REPO="${DEVAPP_REPO:-DingTalk-Real-AI/dingtalk-workspace-cli}"
DEVAPP_VERSION="${DEVAPP_VERSION:-}"
INSTALL_DIR="${DWS_INSTALL_DIR:-$HOME/.local/bin}"
NO_SKILLS="${DWS_NO_SKILLS:-0}"
SKILL_NAME="dingtalk-misc"

say() { printf '  %s\n' "$@"; }
err() { printf '  ❌ %s\n' "$@" >&2; exit 1; }
need_cmd() { command -v "$1" >/dev/null 2>&1 || err "Missing required command: $1"; }

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then openssl dgst -sha256 "$1" | awk '{print $NF}'
  else return 1
  fi
}

verify_release_asset() {
  name="$1"; file="$2"
  checksums="$tmp/checksums.txt"
  if [ ! -f "$checksums" ]; then
    curl -fsSL "https://github.com/${DEVAPP_REPO}/releases/download/${DEVAPP_VERSION}/checksums.txt" -o "$checksums" \
      || err "Could not download checksums.txt; refusing unverified release assets."
  fi
  expected="$(awk -v asset="$name" '$2 == asset {print $1; exit}' "$checksums")"
  [ -n "$expected" ] || err "${name} is missing from checksums.txt."
  actual="$(sha256_file "$file")" || err "Could not compute SHA256 for ${name}."
  [ "$actual" = "$expected" ] || err "SHA256 checksum mismatch for ${name}."
  say "✅ SHA256 checksum verified: ${name}"
}

copy_tree() {
  src="$1"; dest="$2"; parent="$(dirname "$dest")"
  mkdir -p "$parent" || return 1
  stage="$(mktemp -d "$parent/.dws-skill.tmp.XXXXXX")" || return 1
  if ! cp -R "$src/." "$stage/"; then rm -rf "$stage"; return 1; fi
  backup="$(backup_skill_dir "$dest")" || { rm -rf "$stage"; return 1; }
  if mv "$stage" "$dest"; then return 0; fi
  rm -rf "$stage"
  if [ -n "$backup" ] && ! mv "$backup" "$dest"; then
    printf '  ❌ Skill rollback failed; backup retained at %s\n' "$backup" >&2
  fi
  return 1
}

backup_skill_dir() {
  victim="$1"
  [ -e "$victim" ] || [ -L "$victim" ] || { printf '\n'; return 0; }
  stamp="$(date -u +%Y%m%d-%H%M%S)"
  name="$(printf '%s' "$victim" | sed 's#[/\\]#-#g; s#^-##')"
  backup_root="$HOME/.dws/skill-backups/$stamp"; backup="$backup_root/$name"; i=0
  while [ -e "$backup" ] || [ -L "$backup" ]; do i=$((i + 1)); backup_root="$HOME/.dws/skill-backups/$stamp-$i"; backup="$backup_root/$name"; done
  mkdir -p "$backup_root" || return 1
  mv "$victim" "$backup" || return 1
  printf '%s\n' "$backup"
}

same_physical_skill() {
  [ -d "$1" ] && [ -d "$2" ] || return 1
  left="$(CDPATH= cd -- "$1" 2>/dev/null && pwd -P)" || return 1
  right="$(CDPATH= cd -- "$2" 2>/dev/null && pwd -P)" || return 1
  [ "$left" = "$right" ]
}

is_cleanup_only_agent_dir() {
  case "$1" in
    .config/agents/skills|.gemini/antigravity/skills|.gemini/antigravity-cli/skills|.codex/skills|.cursor/skills|.deepagents/agent/skills|.firebender/skills|.gemini/skills|.copilot/skills|.config/opencode/skills|.github/skills|.windsurf/skills|.cline/skills|.amp/skills) return 0 ;;
    *) return 1 ;;
  esac
}

agent_skill_dirs() {
  printf '%s\n' \
    .config/agents/skills .gemini/antigravity/skills .gemini/antigravity-cli/skills .codex/skills .cursor/skills .deepagents/agent/skills .firebender/skills .gemini/skills .copilot/skills .config/opencode/skills \
    .aider-desk/skills .astrbot/data/skills .autohand/skills .augment/skills .bob/skills .claude/skills .openclaw/skills .codeartsdoer/skills .codebuddy/skills .codemaker/skills .codestudio/skills .commandcode/skills .continue/skills .snowflake/cortex/skills .config/crush/skills .config/devin/skills .factory/skills .forge/skills .config/goose/skills .grok/skills .hermes/skills .inferencesh/skills .jazz/skills .junie/skills .iflow/skills .kilocode/skills .config/kimchi/harness/skills .kiro/skills .kode/skills .lingma/skills .mcpjam/skills .minimax/skills .vibe/skills .moxby/skills .mux/skills .openhands/skills .ona/skills .pi/agent/skills .qoder/skills .qoder-cn/skills .qwen/skills .reasonix/skills .rovodev/skills .roo/skills .tabnine/agent/skills .terramind/skills .tinycloud/skills .trae/skills .trae-cn/skills .codeium/windsurf/skills .zcode/skills .zencoder/skills .neovate/skills .pochi/skills .adal/skills \
    .qoderwork/skills .github/skills .windsurf/skills .cline/skills .amp/skills
}

resolve_agent_skill_base() {
  agent_dir="$1"; base="$HOME/$agent_dir"
  case "$agent_dir" in
    .claude/skills) [ -n "${CLAUDE_CONFIG_DIR:-}" ] && base="$CLAUDE_CONFIG_DIR/skills" ;;
    .codex/skills) [ -n "${CODEX_HOME:-}" ] && base="$CODEX_HOME/skills" ;;
    .hermes/skills) [ -n "${HERMES_HOME:-}" ] && base="$HERMES_HOME/skills" ;;
    .autohand/skills) [ -n "${AUTOHAND_HOME:-}" ] && base="$AUTOHAND_HOME/skills" ;;
    .grok/skills) [ -n "${GROK_HOME:-}" ] && base="$GROK_HOME/skills" ;;
    .vibe/skills) [ -n "${VIBE_HOME:-}" ] && base="$VIBE_HOME/skills" ;;
    .openclaw/skills) for legacy in .openclaw .clawdbot .moltbot; do [ -d "$HOME/$legacy" ] && { base="$HOME/$legacy/skills"; break; }; done ;;
    .config/*) base="${XDG_CONFIG_HOME:-$HOME/.config}/${agent_dir#.config/}" ;;
  esac
  printf '%s\n' "$base"
}

agent_skill_base_detected() {
  agent_dir="$1"; base="$2"
  case "$agent_dir" in
    .config/kimchi/harness/skills|.tabnine/agent/skills) [ -d "$(dirname "$(dirname "$base")")" ] ;;
    .zcode/skills) [ -d "$(dirname "$base")" ] || [ -d "/Applications/ZCode.app" ] ;;
    .minimax/skills) [ -d "$(dirname "$base")" ] || [ -d "/Applications/MiniMax Code.app" ] ;;
    *) [ -d "$(dirname "$base")" ] ;;
  esac
}

link_or_copy_skill() {
  canonical="$1"; src="$2"; dest="$3"
  same_physical_skill "$dest" "$canonical" && return 0
  parent="$(dirname "$dest")"; mkdir -p "$parent" || return 1
  parent_real="$(CDPATH= cd -- "$parent" && pwd -P)" || return 1
  target_real="$(CDPATH= cd -- "$canonical" && pwd -P)" || return 1
  relative="$(awk -v from="$parent_real" -v to="$target_real" 'BEGIN { nf=split(from,f,"/"); nt=split(to,t,"/"); i=1; while(i<=nf&&i<=nt&&f[i]==t[i])i++; out=""; for(j=i;j<=nf;j++)if(f[j]!="")out=out"../"; for(j=i;j<=nt;j++)if(t[j]!="")out=out t[j](j<nt?"/":""); if(out=="")out="."; print out }')"
  stage="$(mktemp -d "$parent/.dws-link.tmp.XXXXXX")" || return 1
  if ! ln -s "$relative" "$stage/skill" 2>/dev/null; then
    rm -rf "$stage"
    if copy_tree "$src" "$dest"; then
      printf '  ℹ️  %s 已自动使用兼容方式安装，可正常使用\n' "$dest" >&2
      return 0
    fi
    return 1
  fi
  backup="$(backup_skill_dir "$dest")" || { rm -rf "$stage"; return 1; }
  if mv "$stage/skill" "$dest"; then rm -rf "$stage"; return 0; fi
  rm -rf "$stage"
  if [ -n "$backup" ] && ! mv "$backup" "$dest"; then
    printf '  ❌ Skill rollback failed; backup retained at %s\n' "$backup" >&2
  fi
  return 1
}

need_cmd curl

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo linux ;;
    Darwin*) echo darwin ;;
    MINGW*|MSYS*|CYGWIN*) echo windows ;;  # Git Bash / MSYS2 / Cygwin on Windows
    *) err "Unsupported OS: $(uname -s). On native Windows use install-devapp.ps1." ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) err "Unsupported architecture: $(uname -m)" ;;
  esac
}

# Read the releases list (newest first) and take the top tag, so this also works
# if a release is ever published as a prerelease (which /releases/latest skips).
# Prefer `gh` CLI (authenticated, 5 000 req/h) over raw curl (60 req/h, easily rate-limited).
resolve_version() {
  [ -n "$DEVAPP_VERSION" ] && return 0

  # Try gh CLI first (authenticated, much higher rate limit)
  if command -v gh >/dev/null 2>&1; then
    DEVAPP_VERSION="$(gh api "repos/${DEVAPP_REPO}/releases?per_page=1" --jq '.[0].tag_name' 2>/dev/null || true)"
    [ -n "$DEVAPP_VERSION" ] && return 0
  fi

  # Fallback: unauthenticated curl (may be rate-limited)
  _tmpfile="$(mktemp)"
  _http_code="$(curl -sSL -o "$_tmpfile" -w '%{http_code}' "https://api.github.com/repos/${DEVAPP_REPO}/releases?per_page=1" 2>/dev/null || echo "000")"

  if [ "$_http_code" = "403" ] || [ "$_http_code" = "429" ]; then
    rm -f "$_tmpfile"
    err "GitHub API rate limit hit (HTTP ${_http_code}). Install the GitHub CLI (gh) or set DEVAPP_VERSION explicitly."
  fi

  DEVAPP_VERSION="$(grep -m1 '"tag_name"' "$_tmpfile" | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  rm -f "$_tmpfile"
  [ -n "$DEVAPP_VERSION" ] || err "No release found on ${DEVAPP_REPO}. Set DEVAPP_VERSION to a published release tag."
}

install_skill() {
  bundle="$1"   # extracted skills bundle dir
  src=""
  for c in "$bundle/multi/$SKILL_NAME" "$bundle/skills/multi/$SKILL_NAME" "$bundle/$SKILL_NAME"; do
    [ -f "$c/SKILL.md" ] && src="$c" && break
  done
  [ -n "$src" ] || { say "  (dingtalk-misc not found in skills bundle; skipped)"; return 0; }

  # cache so `dws skill setup --mode multi` can find a source later
  cache="$HOME/.dws/skills/multi/$SKILL_NAME"
  rm -rf "$cache"; mkdir -p "$cache"; cp -R "$src/." "$cache/"

  canonical="$HOME/.agents/skills/$SKILL_NAME"
  copy_tree "$src" "$canonical" || return 1
  installed=1
  failed=0
  for agent_dir in $(agent_skill_dirs); do
    base="$(resolve_agent_skill_base "$agent_dir")"
    agent_skill_base_detected "$agent_dir" "$base" || continue
    same_physical_skill "$base" "$HOME/.agents/skills" && continue
    if is_cleanup_only_agent_dir "$agent_dir"; then
      if ! backup_skill_dir "$base/$SKILL_NAME" >/dev/null; then
        printf '  ⚠️  Agent Skill 旧副本备份失败，保留原目录: %s\n' "$base/$SKILL_NAME" >&2
        failed=$((failed + 1))
      fi
      continue
    fi
    # Per-agent degrade like install.sh: a failed target is reported and
    # skipped loudly instead of aborting the remaining agents mid-loop.
    if ! link_or_copy_skill "$canonical" "$src" "$base/$SKILL_NAME"; then
      printf '  ⚠️  Agent 目标安装失败，已跳过: %s\n' "$base/$SKILL_NAME" >&2
      failed=$((failed + 1))
      continue
    fi
    installed=$((installed + 1))
  done
  if [ "$failed" -gt 0 ]; then
    printf '  ⚠️  有 %s 个 Agent 目标安装 %s 失败\n' "$failed" "$SKILL_NAME" >&2
    return 1
  fi
  say "✅ Skill dingtalk-misc → ${installed} agent dir(s)"
}

main() {
  resolve_version
  os="$(detect_os)"; arch="$(detect_arch)"
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT INT TERM

  printf '\n'
  say "dws dev installer (pre-built binary)"
  say "Repo:    ${DEVAPP_REPO}"
  say "Version: ${DEVAPP_VERSION}"
  say "Target:  ${os}/${arch}"
  printf '\n'

  # 1) binary (already ad-hoc signed by CI; copy does not break the signature)
  if [ "$os" = "windows" ]; then
    asset="dws-windows-${arch}.zip"; binname="dws.exe"
  else
    asset="dws-${os}-${arch}.tar.gz"; binname="dws"
  fi
  say "⬇  Downloading ${asset} ..."
  curl -fsSL "https://github.com/${DEVAPP_REPO}/releases/download/${DEVAPP_VERSION}/${asset}" -o "$tmp/$asset" \
    || err "Binary download failed — does release ${DEVAPP_VERSION} have ${asset}?"
  verify_release_asset "$asset" "$tmp/$asset"
  if [ "$os" = "windows" ]; then
    need_cmd unzip; unzip -q "$tmp/$asset" -d "$tmp"
  else
    need_cmd tar; tar -xzf "$tmp/$asset" -C "$tmp"
  fi
  [ -f "$tmp/$binname" ] || err "${binname} not found inside ${asset}"
  mkdir -p "$INSTALL_DIR"
  cp "$tmp/$binname" "$INSTALL_DIR/$binname"; chmod +x "$INSTALL_DIR/$binname" 2>/dev/null || true
  say "✅ Binary → ${INSTALL_DIR}/${binname}"

  # 2) dev skill from the release's skills bundle
  if [ "$NO_SKILLS" != "1" ]; then
    if curl -fsSL "https://github.com/${DEVAPP_REPO}/releases/download/${DEVAPP_VERSION}/dws-skills.zip" -o "$tmp/skills.zip" 2>/dev/null; then
      verify_release_asset "dws-skills.zip" "$tmp/skills.zip"
      mkdir -p "$tmp/sk"
      if command -v unzip >/dev/null 2>&1; then unzip -q "$tmp/skills.zip" -d "$tmp/sk"; else tar -xf "$tmp/skills.zip" -C "$tmp/sk"; fi
      say ""
      install_skill "$tmp/sk" || err "dingtalk-misc Skill 安装失败，详见上方告警"
    else
      say "  (no dws-skills.zip in release ${DEVAPP_VERSION}; skill skipped)"
    fi
  fi

  printf '\n'
  say "🎉 Done. Next steps:"
  say "  dws version"
  say "  dws auth login"
  say "  dws dev --help --format json"
  say "  dws dev app list --format json"
  printf '\n'
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) say "Note: ${INSTALL_DIR} is not on \$PATH — add it so 'dws' is found." ;;
  esac
}

main
