#!/bin/sh
set -eu

# Install DWS agent skills from GitHub Releases into agent skill directories.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/DingTalk-Real-AI/dingtalk-workspace-cli/main/scripts/install-skills.sh | sh
#
# Downloads dws-skills.zip from GitHub Releases and copies it under each target
# path using the same rules as build/npm/install.js installSkillsToHomes
# (AGENT_DIRS + parent-directory gate), with root defaulting to the current
# directory. Set DWS_SKILLS_ROOT=$HOME to match npm install layout exactly.
#
# Environment variables (optional):
#   DWS_VERSION        — release tag (default: latest)
#   DWS_SKILLS_ROOT    — base path for agent dirs (default: $PWD)
#   DWS_SKILL_MODE     — mono | multi (default: multi)
#   DWS_GITEE_REPO     — "owner/repo" on Gitee; resolve version + assets via the
#                        Gitee API instead of GitHub (China mirror)

REPO="DingTalk-Real-AI/dingtalk-workspace-cli"
# China mirror: Gitee repo "owner/repo". When set, version + asset URLs resolve via Gitee API.
GITEE_REPO="${DWS_GITEE_REPO:-}"
# Auto-fallback Gitee mirror used when GitHub is unreachable (see pick_source).
GITEE_FALLBACK_REPO="${DWS_GITEE_FALLBACK_REPO:-DingTalk-Real-AI/dingtalk-workspace-cli}"
VERSION="${DWS_VERSION:-latest}"
SKILL_NAME="dws"
ROOT="${DWS_SKILLS_ROOT:-$PWD}"
DWS_CACHE_ROOT="${DWS_CACHE_ROOT:-$HOME/.dws}"
SKILL_MODE="$(printf '%s' "${DWS_SKILL_MODE:-multi}" | tr '[:upper:]' '[:lower:]')"
case "$SKILL_MODE" in
  mono|multi) ;;
  *) printf '❌ Invalid DWS_SKILL_MODE=%s. Use mono or multi.\n' "${DWS_SKILL_MODE:-}" >&2; exit 1 ;;
esac

# ── Helpers ──────────────────────────────────────────────────────────────────

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf '❌ Missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

# backup_and_remove_skill_dir <dir>
# Moves <dir> into $HOME/.dws/skill-backups/<stamp>/<name> instead of
# destroying it (non-interactive installs cannot confirm, so removals must
# stay reversible). Missing paths are a no-op success. On any backup failure
# the directory is left in place and a non-zero status is returned so callers
# skip that target rather than silently deleting data.
backup_and_remove_skill_dir() {
  _bed_dir="$1"
  [ -d "$_bed_dir" ] || return 0
  _bed_root="${HOME}/.dws/skill-backups"
  _bed_stamp="$(date -u +%Y%m%d-%H%M%S)"
  _bed_name="$(basename "$_bed_dir")"
  _bed_target="$_bed_root/$_bed_stamp/$_bed_name"
  _bed_i=1
  while [ -e "$_bed_target" ]; do
    _bed_target="$_bed_root/$_bed_stamp-$_bed_i/$_bed_name"
    _bed_i=$((_bed_i + 1))
    if [ "$_bed_i" -gt 1000 ]; then
      printf '  ⚠️  备份目录冲突，保留原目录 %s\n' "$_bed_dir"
      return 1
    fi
  done
  mkdir -p "$(dirname "$_bed_target")" 2>/dev/null || {
    printf '  ⚠️  无法创建备份目录，保留原目录 %s\n' "$_bed_dir"
    return 1
  }
  if mv "$_bed_dir" "$_bed_target" 2>/dev/null; then
    printf '  × 已备份并移除 %s → %s\n' "$_bed_dir" "$_bed_target"
    return 0
  fi
  printf '  ⚠️  备份失败，保留原目录 %s\n' "$_bed_dir"
  return 1
}

# Fetch a Gitee API endpoint, retrying transient 502/503 from Gitee's gateway.
gitee_api() {
  _url="$1"
  _try=1
  while [ "$_try" -le 4 ]; do
    if _resp="$(curl -fsSL "$_url" 2>/dev/null)" && [ -n "$_resp" ]; then
      printf '%s' "$_resp"
      return 0
    fi
    _try=$((_try + 1))
    sleep 2
  done
  return 1
}

pick_source() {
  [ -n "$GITEE_REPO" ] && return 0
  [ "${DWS_NO_FALLBACK:-0}" = "1" ] && return 0
  curl -fsS --connect-timeout 5 --max-time 12 -o /dev/null "https://github.com/${REPO}/releases/latest" 2>/dev/null && return 0
  GITEE_REPO="$GITEE_FALLBACK_REPO"
  printf '  ⚠ GitHub 不可达，自动切换国内 Gitee 镜像: %s\n' "$GITEE_REPO"
}

resolve_version() {
  if [ "$VERSION" = "latest" ]; then
    if [ -n "$GITEE_REPO" ]; then
      # Gitee's /releases/latest and /releases endpoints are unreliable, so
      # resolve the newest vN.N.N tag from the git tags endpoint instead.
      VERSION="$(gitee_api "https://gitee.com/api/v5/repos/${GITEE_REPO}/tags" \
        | grep -o '"name":[ ]*"v[0-9][0-9.]*"' \
        | sed 's/.*"name":[ ]*"//;s/"$//' \
        | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' \
        | sort -V | tail -1)"
    else
      VERSION="$(curl -fsSI "https://github.com/${REPO}/releases/latest" 2>/dev/null \
        | grep -i '^location:' | sed 's|.*/tag/||;s/[[:space:]]*$//')"
    fi
    if [ -z "$VERSION" ]; then
      printf '❌ Could not determine the latest version. Set DWS_VERSION explicitly.\n' >&2
      exit 1
    fi
  fi
}

# Resolve a release asset's download URL by name (GitHub template vs Gitee API).
asset_url() {
  _name="$1"
  if [ -z "$GITEE_REPO" ]; then
    printf '%s' "https://github.com/${REPO}/releases/download/${VERSION}/${_name}"
    return 0
  fi
  gitee_api "https://gitee.com/api/v5/repos/${GITEE_REPO}/releases/tags/${VERSION}" \
    | tr '}' '\n' \
    | grep "\"name\":[ ]*\"${_name}\"" \
    | grep -o '"browser_download_url":[ ]*"[^"]*"' \
    | head -1 | sed 's/.*"browser_download_url":[ ]*"//;s/"$//'
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
  printf '❌ Missing required command: unzip (or tar with zip support)\n' >&2
  exit 1
}

# One-line summary copy (2nd+ targets).
_copy_skill_summary() {
  _src="$1"
  _dest="$2"
  _label="$3"

  if [ -d "$_dest" ]; then
    backup_and_remove_skill_dir "$_dest" || {
      printf '  ⚠️  跳过 %s（保留原目录）\n' "$_dest"
      return 1
    }
  fi

  mkdir -p "$_dest"
  cp -R "$_src/"* "$_dest/" 2>/dev/null || cp -r "$_src/"* "$_dest/"
  file_count="$(find "$_dest" -type f | wc -l | tr -d ' ')"

  printf '  ✅ Skills → %s (%s files)\n' "$_label" "$file_count"
}

# Full copy with top-level listing (1st target).
_copy_skill() {
  _src="$1"
  _dest="$2"
  _label="$3"

  if [ -d "$_dest" ]; then
    backup_and_remove_skill_dir "$_dest" || {
      printf '  ⚠️  跳过 %s（保留原目录）\n' "$_dest"
      return 1
    }
  fi

  mkdir -p "$_dest"
  cp -R "$_src/"* "$_dest/" 2>/dev/null || cp -r "$_src/"* "$_dest/"
  file_count="$(find "$_dest" -type f | wc -l | tr -d ' ')"

  printf '  ✅ Skills → %s (%s files)\n' "$_label" "$file_count"

  for entry in "$_dest"/*; do
    entry_name="$(basename "$entry")"
    if [ -d "$entry" ]; then
      sub_count="$(find "$entry" -type f | wc -l | tr -d ' ')"
      printf '     📁 %s/ (%s files)\n' "$entry_name" "$sub_count"
    else
      printf '     📄 %s\n' "$entry_name"
    fi
  done
}

# multi_tree_has_skills returns 0 only when the given multi bundle directory
# contains at least one product skill (a subdirectory with a SKILL.md). An
# empty or corrupt multi/ tree must never select the multi branch: installing
# it would delete existing dws/ + dingtalk-* skills and lay down nothing.
# (Go bundleSkillNames and install.js multiTreeHasSkills guard the same way.)
multi_tree_has_skills() {
  _dir="$1"
  [ -d "$_dir" ] || return 1
  for _sub in "$_dir"/*/; do
    if [ -f "${_sub}SKILL.md" ]; then
      return 0
    fi
  done
  return 1
}

# Same semantics as build/npm/install.js installMultiSkillsToHomes (root = DWS_SKILLS_ROOT or PWD).
install_multi_skills_to_root() {
  multi_src="$1"
  root="$2"
  installed=0
  attempted=0
  idx=0
  for agent_dir in \
    ".agents/skills" \
    ".claude/skills" \
    ".cursor/skills" \
    ".qoder/skills" \
    ".qoderwork/skills" \
    ".gemini/skills" \
    ".codex/skills" \
    ".github/skills" \
    ".windsurf/skills" \
    ".augment/skills" \
    ".cline/skills" \
    ".amp/skills" \
    ".kiro/skills" \
    ".trae/skills" \
    ".openclaw/skills" \
    ".hermes/skills"
  do
    base_dir="$root/$agent_dir"
    parent_gate="$(dirname "$base_dir")"
    if [ "$idx" -gt 0 ] && [ ! -e "$parent_gate" ]; then
      idx=$((idx + 1))
      continue
    fi
    attempted=$((attempted + 1))
    if _install_multi_to_base "$multi_src" "$base_dir" "$root" "$agent_dir"; then
      installed=$((installed + 1))
    else
      printf '  ⚠️  跳过 %s（备份失败，未安装 multi）\n' "$base_dir"
    fi
    idx=$((idx + 1))
  done
  if [ "$attempted" -eq 0 ] && _install_multi_to_base "$multi_src" "$root/.agents/skills" "$root" ".agents/skills"; then
    installed=$((installed + 1))
  fi
  if [ "$installed" -eq 0 ]; then
    printf '  ⚠️  未安装任何 multi Skill：所有检测到的 Agent 目标均失败\n'
  fi
}

_install_multi_to_base() {
  _msrc="$1"
  _base="$2"
  _root="$3"
  _agent_dir="$4"

  mkdir -p "$_base" || return 1

  # Mutual exclusion: back up + remove the mono leftover.
  backup_and_remove_skill_dir "$_base/$SKILL_NAME" || return 1

  # Back up + remove stale multi skills (dingtalk-* or dws-shared) not in the
  # new bundle.
  for existing in "$_base"/dingtalk-*/; do
    [ -d "$existing" ] || continue
    _name="$(basename "$existing")"
    if [ ! -f "$_msrc/$_name/SKILL.md" ]; then
      backup_and_remove_skill_dir "$existing" || return 1
    fi
  done
  if [ -d "$_base/dws-shared" ] && [ ! -f "$_msrc/dws-shared/SKILL.md" ]; then
    backup_and_remove_skill_dir "$_base/dws-shared" || return 1
  fi

  # Complete all backups before copying the first new skill so a later
  # failure cannot leave a partial multi bundle in this Agent target.
  for skill_dir in "$_msrc"/*/; do
    [ -f "${skill_dir}SKILL.md" ] || continue
    _name="$(basename "$skill_dir")"
    _dest="$_base/$_name"
    backup_and_remove_skill_dir "$_dest" || return 1
  done

  _count=0
  for skill_dir in "$_msrc"/*/; do
    [ -f "${skill_dir}SKILL.md" ] || continue
    _name="$(basename "$skill_dir")"
    _dest="$_base/$_name"
    mkdir -p "$_dest" || return 1
    cp -R "${skill_dir}." "$_dest/" 2>/dev/null || cp -r "${skill_dir}." "$_dest/"
    _count=$((_count + 1))
  done

  if [ "$_root" = "$HOME" ]; then
    _label="~/$_agent_dir/"
  else
    _label="$_root/$_agent_dir/"
  fi
  printf '  ✅ Skills → %s (%s product skills)\n' "$_label" "$_count"
}

# Same semantics as build/npm/install.js installSkillsToHomes (root = DWS_SKILLS_ROOT or PWD).
install_skills_to_root() {
  skill_src="$1"
  root="$2"
  installed=0
  attempted=0
  idx=0
  for agent_dir in \
    ".agents/skills" \
    ".claude/skills" \
    ".cursor/skills" \
    ".qoder/skills" \
    ".qoderwork/skills" \
    ".gemini/skills" \
    ".codex/skills" \
    ".github/skills" \
    ".windsurf/skills" \
    ".augment/skills" \
    ".cline/skills" \
    ".amp/skills" \
    ".kiro/skills" \
    ".trae/skills" \
    ".openclaw/skills" \
    ".hermes/skills"
  do
    base_dir="$root/$agent_dir"
    parent_gate="$(dirname "$base_dir")"
    if [ "$idx" -gt 0 ] && [ ! -e "$parent_gate" ]; then
      idx=$((idx + 1))
      continue
    fi
    attempted=$((attempted + 1))
    # Mutual exclusion: back up + remove multi leftovers before laying down
    # mono. Non-interactive installs cannot confirm, so removals stay
    # reversible via ~/.dws/skill-backups/ (backup failure keeps the dir).
    cleanup_ok=1
    backup_and_remove_skill_dir "$base_dir/dws-shared" || cleanup_ok=0
    if [ "$cleanup_ok" -ne 1 ]; then
      printf '  ⚠️  跳过 %s（multi 残留备份失败，未安装 mono）\n' "$base_dir"
      idx=$((idx + 1))
      continue
    fi
    for existing in "$base_dir"/dingtalk-*/; do
      [ -d "$existing" ] || continue
      backup_and_remove_skill_dir "$existing" || {
        cleanup_ok=0
        break
      }
    done
    if [ "$cleanup_ok" -ne 1 ]; then
      printf '  ⚠️  跳过 %s（multi 残留备份失败，未安装 mono）\n' "$base_dir"
      idx=$((idx + 1))
      continue
    fi
    dest="$base_dir/$SKILL_NAME"
    if [ "$root" = "$HOME" ]; then
      label="~/$agent_dir/$SKILL_NAME"
    else
      label="$root/$agent_dir/$SKILL_NAME"
    fi
    if [ "$installed" -eq 0 ]; then
      if _copy_skill "$skill_src" "$dest" "$label"; then
        installed=$((installed + 1))
      fi
    elif _copy_skill_summary "$skill_src" "$dest" "$label"; then
      installed=$((installed + 1))
    fi
    idx=$((idx + 1))
  done
  if [ "$attempted" -eq 0 ]; then
    if [ "$root" = "$HOME" ]; then
      flabel="~/.agents/skills/$SKILL_NAME"
    else
      flabel="$root/.agents/skills/$SKILL_NAME"
    fi
    if _copy_skill "$skill_src" "$root/.agents/skills/$SKILL_NAME" "$flabel"; then
      installed=$((installed + 1))
    fi
  fi
  if [ "$installed" -eq 0 ]; then
    printf '  ⚠️  未安装任何 mono Skill：所有检测到的 Agent 目标均失败\n'
  fi
}

# ── Main ─────────────────────────────────────────────────────────────────────

main() {
  need_cmd curl
  pick_source
  resolve_version

  printf '\n'
  printf '  ┌──────────────────────────────────────┐\n'
  printf '  │     DWS Skill Installer              │\n'
  printf '  │     DingTalk Workspace CLI            │\n'
  printf '  └──────────────────────────────────────┘\n'
  printf '\n'

  TMPDIR_WORK="$(mktemp -d)"
  trap 'rm -rf "$TMPDIR_WORK"' EXIT INT TERM

  ASSET_URL="$(asset_url dws-skills.zip)"
  [ -n "$ASSET_URL" ] || { printf '❌ Could not resolve download URL for dws-skills.zip (version %s).\n' "$VERSION" >&2; exit 1; }
  printf '  ⬇  Downloading skills from GitHub Releases: %s (%s)\n' "$REPO" "$VERSION"
  curl -fsSL "$ASSET_URL" -o "$TMPDIR_WORK/dws-skills.zip"
  extract_zip "$TMPDIR_WORK/dws-skills.zip" "$TMPDIR_WORK/extracted"

  # Prefer the explicit mono/ subtree; fall back to legacy nested or zip root.
  SKILL_SRC="$TMPDIR_WORK/extracted"
  if [ -d "$TMPDIR_WORK/extracted/mono" ] && [ -f "$TMPDIR_WORK/extracted/mono/SKILL.md" ]; then
    SKILL_SRC="$TMPDIR_WORK/extracted/mono"
  elif [ -f "$TMPDIR_WORK/extracted/${SKILL_NAME}/SKILL.md" ]; then
    SKILL_SRC="$TMPDIR_WORK/extracted/${SKILL_NAME}"
  fi

  printf '\n'
  printf '  Installing under root: %s (mode: %s)\n' "$ROOT" "$SKILL_MODE"
  # Multi first: a release may ship only the multi/ tree without the root
  # mono copy, so the mono SKILL.md gate must never block a multi install.
  # An empty/corrupt multi/ tree (no */SKILL.md) falls back to mono with a
  # warning — installing it would wipe existing skills and lay down nothing.
  if [ "$SKILL_MODE" = "multi" ] && multi_tree_has_skills "$TMPDIR_WORK/extracted/multi"; then
    install_multi_skills_to_root "$TMPDIR_WORK/extracted/multi" "$ROOT"
  else
    if [ "$SKILL_MODE" = "multi" ]; then
      printf '  ⚠️  Multi skill tree not found or empty in release asset; falling back to mono.\n'
    fi
    if [ ! -f "$SKILL_SRC/SKILL.md" ]; then
      printf '  ❌ Skill source not found in release asset\n' >&2
      exit 1
    fi
    install_skills_to_root "$SKILL_SRC" "$ROOT"
  fi

  # Cache multi/ (and a mono copy) under ~/.dws/skills so that subsequent
  # `dws skill setup --mode multi|mono` invocations can find a source. An
  # empty/corrupt tree must never wipe a previously good cache.
  if multi_tree_has_skills "$TMPDIR_WORK/extracted/multi"; then
    cache_dir="${DWS_CACHE_ROOT}/skills/multi"
    rm -rf "$cache_dir"
    mkdir -p "$cache_dir"
    cp -R "$TMPDIR_WORK/extracted/multi/"* "$cache_dir/" 2>/dev/null || \
      cp -r "$TMPDIR_WORK/extracted/multi/"* "$cache_dir/" 2>/dev/null || true
    file_count="$(find "$cache_dir" -type f | wc -l | tr -d ' ')"
    printf '  ✅ Cached multi skills → %s (%s files)\n' "$cache_dir" "$file_count"
  fi
  if [ -f "$SKILL_SRC/SKILL.md" ]; then
    mono_cache="${DWS_CACHE_ROOT}/skills/mono"
    rm -rf "$mono_cache"
    mkdir -p "$mono_cache"
    cp -R "$SKILL_SRC/"* "$mono_cache/" 2>/dev/null || \
      cp -r "$SKILL_SRC/"* "$mono_cache/" 2>/dev/null || true
  fi

  printf '\n'
  printf '  📖 Skill includes:\n'
  printf '     • SKILL.md — Main skill with product overview and intent routing\n'
  printf '     • references/ — Detailed product command references\n'
  printf '     • scripts/ — Batch operation scripts for all products\n'
  printf '\n'
  printf '  ⚡ Requires: dws CLI installed and on $PATH\n'
  printf '     Install: go install github.com/%s/cmd@latest\n' "$REPO"
  printf '\n'
}

main
