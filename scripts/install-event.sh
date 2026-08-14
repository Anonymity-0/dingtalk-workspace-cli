#!/bin/sh
# Copyright 2026 Alibaba Group
# Licensed under the Apache License, Version 2.0
#
# One-command installer for dws personal events.
# Downloads the official dws binary and installs:
#   - multi skill: dingtalk-event (personal IM/OA event routing)
#   - multi prerequisites: dingtalk-shared + clean dingtalk-misc
#   - mono skill:  dws
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/DingTalk-Real-AI/dingtalk-workspace-cli/main/scripts/install-event.sh | sh
#
# Env (all optional):
#   EVENT_REPO       repo holding official releases (default: DingTalk-Real-AI/dingtalk-workspace-cli)
#   EVENT_VERSION    pin a release tag (default: latest stable release)
#   DWS_VERSION      alias for EVENT_VERSION when EVENT_VERSION is empty
#   DWS_INSTALL_DIR  binary dir (default: ~/.local/bin)
#   DWS_NO_SKILLS    set 1 to skip skill installation
#   DWS_SKILLS_ONLY  set 1 to install only skills, without touching the binary
set -eu

EVENT_REPO="${EVENT_REPO:-DingTalk-Real-AI/dingtalk-workspace-cli}"
EVENT_VERSION="${EVENT_VERSION:-${DWS_VERSION:-}}"
INSTALL_DIR="${DWS_INSTALL_DIR:-$HOME/.local/bin}"
NO_SKILLS="${DWS_NO_SKILLS:-0}"
SKILLS_ONLY="${DWS_SKILLS_ONLY:-0}"
BIN_NAME="dws"
EVENT_SKILL_NAME="dingtalk-event"
SHARED_SKILL_NAME="dingtalk-shared"
MISC_SKILL_NAME="dingtalk-misc"
MONO_SKILL_NAME="dws"

if [ "$EVENT_VERSION" = "latest" ]; then
  EVENT_VERSION=""
fi

say() { printf '  %s\n' "$@"; }
err() { printf '  ERROR: %s\n' "$@" >&2; exit 1; }
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
    curl -fsSL "https://github.com/${EVENT_REPO}/releases/download/${EVENT_VERSION}/checksums.txt" -o "$checksums" \
      || err "Could not download checksums.txt; refusing unverified release assets."
  fi
  expected="$(awk -v asset="$name" '$2 == asset {print $1; exit}' "$checksums")"
  [ -n "$expected" ] || err "${name} is missing from checksums.txt."
  actual="$(sha256_file "$file")" || err "Could not compute SHA256 for ${name}."
  [ "$actual" = "$expected" ] || err "SHA256 checksum mismatch for ${name}."
  say "SHA256 checksum verified: ${name}"
}

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo linux ;;
    Darwin*) echo darwin ;;
    MINGW*|MSYS*|CYGWIN*) echo windows ;;
    *) err "Unsupported OS: $(uname -s). On native Windows use a PowerShell installer." ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) err "Unsupported architecture: $(uname -m)" ;;
  esac
}

extract_zip() {
  archive="$1"
  dest="$2"
  if command -v unzip >/dev/null 2>&1; then
    unzip -q "$archive" -d "$dest"
    return 0
  fi
  if command -v tar >/dev/null 2>&1 && tar -xf "$archive" -C "$dest" >/dev/null 2>&1; then
    return 0
  fi
  err "Missing required command: unzip (or tar with zip support)"
}

resolve_event_version() {
  [ -n "$EVENT_VERSION" ] && return 0

  if command -v gh >/dev/null 2>&1; then
    EVENT_VERSION="$(gh api "repos/${EVENT_REPO}/releases/latest" --jq '.tag_name' 2>/dev/null || true)"
    [ -n "$EVENT_VERSION" ] && [ "$EVENT_VERSION" != "null" ] && return 0
    EVENT_VERSION=""
  fi

  tmpfile="$(mktemp)"
  http_code="$(curl -sSL -o "$tmpfile" -w '%{http_code}' "https://api.github.com/repos/${EVENT_REPO}/releases/latest" 2>/dev/null || echo "000")"
  if [ "$http_code" = "403" ] || [ "$http_code" = "429" ]; then
    rm -f "$tmpfile"
    err "GitHub API rate limit hit (HTTP ${http_code}). Install gh or set EVENT_VERSION explicitly."
  fi
  EVENT_VERSION="$(grep -m1 '"tag_name"' "$tmpfile" | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' || true)"
  rm -f "$tmpfile"
  [ -n "$EVENT_VERSION" ] || err "No stable release found on ${EVENT_REPO}. Set EVENT_VERSION to a published release tag."
}

copy_tree() {
  src="$1"
  dest="$2"
  parent="$(dirname "$dest")"
  mkdir -p "$parent" || return 1
  stage="$(mktemp -d "$parent/.dws-skill.tmp.XXXXXX")" || return 1
  if ! cp -R "$src/." "$stage/"; then rm -rf "$stage"; return 1; fi
  backup="$(backup_skill_dir "$dest")" || { rm -rf "$stage"; return 1; }
  if mv "$stage" "$dest"; then return 0; fi
  rm -rf "$stage"
  [ -z "$backup" ] || mv "$backup" "$dest" 2>/dev/null || true
  return 1
}

backup_skill_dir() {
  victim="$1"
  [ -e "$victim" ] || [ -L "$victim" ] || { printf '\n'; return 0; }
  stamp="$(date -u +%Y%m%d-%H%M%S)"
  name="$(printf '%s' "$victim" | sed 's#[/\\]#-#g; s#^-##')"
  backup_root="$HOME/.dws/skill-backups/$stamp"
  backup="$backup_root/$name"
  i=0
  while [ -e "$backup" ] || [ -L "$backup" ]; do
    i=$((i + 1)); backup_root="$HOME/.dws/skill-backups/$stamp-$i"; backup="$backup_root/$name"
  done
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
  if ! ln -s "$relative" "$stage/skill" 2>/dev/null; then rm -rf "$stage"; copy_tree "$src" "$dest"; return; fi
  backup="$(backup_skill_dir "$dest")" || { rm -rf "$stage"; return 1; }
  if mv "$stage/skill" "$dest"; then rm -rf "$stage"; return 0; fi
  rm -rf "$stage"; [ -z "$backup" ] || mv "$backup" "$dest" 2>/dev/null || true
  return 1
}

install_skill_to_homes() {
  src="$1"
  skill_name="$2"
  canonical="$HOME/.agents/skills/$skill_name"
  copy_tree "$src" "$canonical" || return 1
  installed=1
  for agent_dir in $(agent_skill_dirs); do
    base="$(resolve_agent_skill_base "$agent_dir")"
    agent_skill_base_detected "$agent_dir" "$base" || continue
    same_physical_skill "$base" "$HOME/.agents/skills" && continue
    if is_cleanup_only_agent_dir "$agent_dir"; then backup_skill_dir "$base/$skill_name" >/dev/null || return 1; continue; fi
    link_or_copy_skill "$canonical" "$src" "$base/$skill_name" || return 1
    installed=$((installed + 1))
  done
  printf '%s\n' "$installed"
}

find_multi_skill_src() {
  bundle="$1"
  skill_name="$2"
  for c in \
    "$bundle/multi/$skill_name" \
    "$bundle/skills/multi/$skill_name" \
    "$bundle/$skill_name"
  do
    if [ -f "$c/SKILL.md" ]; then
      printf '%s\n' "$c"
      return 0
    fi
  done
  return 1
}

find_mono_skill_src() {
  bundle="$1"
  for c in \
    "$bundle/mono" \
    "$bundle/skills/mono" \
    "$bundle/$MONO_SKILL_NAME" \
    "$bundle"
  do
    if [ -f "$c/SKILL.md" ]; then
      printf '%s\n' "$c"
      return 0
    fi
  done
  return 1
}

install_skills_from_bundle() {
  bundle="$1"

  # Resolve the complete, version-matched set before touching any installed or
  # cached skill. A malformed release therefore cannot leave a half-migrated
  # event/misc pair behind.
  event_src="$(find_multi_skill_src "$bundle" "$EVENT_SKILL_NAME" || true)"
  [ -n "$event_src" ] || err "${EVENT_SKILL_NAME} not found in dws-skills.zip"
  shared_src="$(find_multi_skill_src "$bundle" "$SHARED_SKILL_NAME" || true)"
  [ -n "$shared_src" ] || err "${SHARED_SKILL_NAME} not found in dws-skills.zip"
  misc_src="$(find_multi_skill_src "$bundle" "$MISC_SKILL_NAME" || true)"
  [ -n "$misc_src" ] || err "${MISC_SKILL_NAME} not found in dws-skills.zip"
  mono_src="$(find_mono_skill_src "$bundle" || true)"
  [ -n "$mono_src" ] || err "mono ${MONO_SKILL_NAME} skill not found in dws-skills.zip"

  event_cache="$HOME/.dws/skills/multi/$EVENT_SKILL_NAME"
  shared_cache="$HOME/.dws/skills/multi/$SHARED_SKILL_NAME"
  misc_cache="$HOME/.dws/skills/multi/$MISC_SKILL_NAME"
  copy_tree "$event_src" "$event_cache"
  copy_tree "$shared_src" "$shared_cache"
  copy_tree "$misc_src" "$misc_cache"

  mono_cache="$HOME/.dws/skills/mono"
  copy_tree "$mono_src" "$mono_cache"

  event_installed="$(install_skill_to_homes "$event_src" "$EVENT_SKILL_NAME")"
  shared_installed="$(install_skill_to_homes "$shared_src" "$SHARED_SKILL_NAME")"
  misc_installed="$(install_skill_to_homes "$misc_src" "$MISC_SKILL_NAME")"
  mono_installed="$(install_skill_to_homes "$mono_src" "$MONO_SKILL_NAME")"

  say "Skill ${EVENT_SKILL_NAME} -> ${event_installed} agent dir(s)"
  say "Skill ${SHARED_SKILL_NAME} -> ${shared_installed} agent dir(s)"
  say "Skill ${MISC_SKILL_NAME} -> ${misc_installed} agent dir(s)"
  say "Skill ${MONO_SKILL_NAME} -> ${mono_installed} agent dir(s)"
  say "Cached ${EVENT_SKILL_NAME} -> ${event_cache}"
  say "Cached ${SHARED_SKILL_NAME} -> ${shared_cache}"
  say "Cached ${MISC_SKILL_NAME} -> ${misc_cache}"
  say "Cached mono ${MONO_SKILL_NAME} -> ${mono_cache}"
}

install_binary() {
  os="$(detect_os)"
  arch="$(detect_arch)"
  if [ "$os" = "windows" ]; then
    asset="${BIN_NAME}-windows-${arch}.zip"
    binname="${BIN_NAME}.exe"
  else
    asset="${BIN_NAME}-${os}-${arch}.tar.gz"
    binname="${BIN_NAME}"
  fi

  say "Downloading ${asset} ..."
  curl -fsSL "https://github.com/${EVENT_REPO}/releases/download/${EVENT_VERSION}/${asset}" -o "$tmp/$asset" \
    || err "Binary download failed. Does release ${EVENT_VERSION} have ${asset}?"
  verify_release_asset "$asset" "$tmp/$asset"

  if [ "$os" = "windows" ]; then
    extract_zip "$tmp/$asset" "$tmp/bin"
  else
    need_cmd tar
    mkdir -p "$tmp/bin"
    tar -xzf "$tmp/$asset" -C "$tmp/bin"
  fi

  found=""
  for c in "$tmp/bin/$binname" "$tmp/bin/${BIN_NAME}-${os}-${arch}/$binname"; do
    [ -f "$c" ] && found="$c" && break
  done
  if [ -z "$found" ]; then
    found="$(find "$tmp/bin" -name "$binname" -type f | head -1 || true)"
  fi
  [ -n "$found" ] || err "${binname} not found inside ${asset}"

  mkdir -p "$INSTALL_DIR"
  cp "$found" "$INSTALL_DIR/$binname"
  chmod +x "$INSTALL_DIR/$binname" 2>/dev/null || true
  say "Binary -> ${INSTALL_DIR}/${binname}"
}

install_skills() {
  say "Downloading dws-skills.zip ..."
  curl -fsSL "https://github.com/${EVENT_REPO}/releases/download/${EVENT_VERSION}/dws-skills.zip" -o "$tmp/dws-skills.zip" \
    || err "dws-skills.zip download failed for release ${EVENT_VERSION}"
  verify_release_asset "dws-skills.zip" "$tmp/dws-skills.zip"

  mkdir -p "$tmp/skills"
  extract_zip "$tmp/dws-skills.zip" "$tmp/skills"
  install_skills_from_bundle "$tmp/skills"
}

main() {
  need_cmd curl
  if [ "$NO_SKILLS" = "1" ] && [ "$SKILLS_ONLY" = "1" ]; then
    err "DWS_NO_SKILLS=1 cannot be combined with DWS_SKILLS_ONLY=1"
  fi

  resolve_event_version
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT INT TERM

  printf '\n'
  say "dws event installer"
  say "Repo:    ${EVENT_REPO}"
  say "Version: ${EVENT_VERSION}"
  say "Install: ${INSTALL_DIR}"
  printf '\n'

  if [ "$SKILLS_ONLY" != "1" ]; then
    install_binary
  else
    say "DWS_SKILLS_ONLY=1, skipping binary installation."
  fi

  if [ "$NO_SKILLS" != "1" ]; then
    install_skills
  else
    say "DWS_NO_SKILLS=1, skipping skill installation."
  fi

  printf '\n'
  say "Done. Verify with:"
  if [ "$SKILLS_ONLY" != "1" ]; then
    say "  dws version"
  fi
  say "  dws event schema user_im_message_receive_o2o"
  say "  dws event consume user_im_message_receive_o2o --user <userId> -f ndjson"
  say ""
  say "If an Agent session is already open, restart it or reload skills before testing event routing."
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) [ "$SKILLS_ONLY" = "1" ] || say "Note: ${INSTALL_DIR} is not on PATH." ;;
  esac
}

main
