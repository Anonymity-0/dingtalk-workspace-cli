#!/usr/bin/env node

"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");
const childProcess = require("child_process");

// Canonical list: keep scripts/install.sh, scripts/install.ps1, scripts/install-skills.sh in sync.
const AGENT_DIRS = [
  ".agents/skills",
  ".claude/skills",
  ".cursor/skills",
  ".qoder/skills",
  ".qoderwork/skills",
  ".gemini/skills",
  ".codex/skills",
  ".github/skills",
  ".windsurf/skills",
  ".augment/skills",
  ".cline/skills",
  ".amp/skills",
  ".kiro/skills",
  ".trae/skills",
  ".openclaw/skills",
  ".hermes/skills",
];

const PLATFORM_MAP = {
  "darwin-x64": "dws-darwin-amd64.tar.gz",
  "darwin-arm64": "dws-darwin-arm64.tar.gz",
  "linux-x64": "dws-linux-amd64.tar.gz",
  "linux-arm64": "dws-linux-arm64.tar.gz",
  "win32-x64": "dws-windows-amd64.zip",
  "win32-arm64": "dws-windows-arm64.zip",
};

function run(command, args) {
  childProcess.execFileSync(command, args, { stdio: "inherit" });
}

function ensureCleanDir(dir) {
  fs.rmSync(dir, { recursive: true, force: true });
  fs.mkdirSync(dir, { recursive: true });
}

// backupStamp returns the UTC timestamp used for backup directory names,
// matching the shell installers' `date -u +%Y%m%d-%H%M%S` layout.
function backupStamp() {
  const d = new Date();
  const pad = (n) => String(n).padStart(2, "0");
  return (
    `${d.getUTCFullYear()}${pad(d.getUTCMonth() + 1)}${pad(d.getUTCDate())}` +
    `-${pad(d.getUTCHours())}${pad(d.getUTCMinutes())}${pad(d.getUTCSeconds())}`
  );
}

// backupAndRemoveSkillDir moves dir into <homeDir>/.dws/skill-backups/
// <stamp>/<rel-or-basename> instead of destroying it (non-interactive
// installs cannot confirm, so removals must stay reversible). Missing paths
// are a no-op success. On any backup failure the directory is left in place
// and false is returned so callers skip that target rather than silently
// deleting data.
function backupAndRemoveSkillDir(homeDir, dir) {
  if (!fs.existsSync(dir) || !fs.statSync(dir).isDirectory()) {
    return true;
  }
  const rel = path.relative(homeDir, dir);
  const name =
    rel && rel !== "." && !rel.startsWith("..") && !path.isAbsolute(rel)
      ? rel.split(path.sep).join("-")
      : path.basename(dir);
  const stamp = backupStamp();
  const backupRoot = path.join(homeDir, ".dws", "skill-backups");
  let targetRoot = path.join(backupRoot, stamp);
  let target = path.join(targetRoot, name);
  for (let i = 1; fs.existsSync(target); i++) {
    if (i > 1000) {
      console.warn(`⚠️  备份目录冲突，保留原目录 ${dir}`);
      return false;
    }
    targetRoot = path.join(backupRoot, `${stamp}-${i}`);
    target = path.join(targetRoot, name);
  }
  try {
    fs.mkdirSync(targetRoot, { recursive: true });
    fs.renameSync(dir, target);
  } catch (err) {
    console.warn(`⚠️  备份失败，保留原目录 ${dir}: ${err.message}`);
    return false;
  }
  console.log(`  × 已备份并移除 ${dir} → ${target}`);
  return true;
}

function findBinary(root) {
  const entries = fs.readdirSync(root, { withFileTypes: true });
  for (const entry of entries) {
    const entryPath = path.join(root, entry.name);
    if (entry.isDirectory()) {
      const nested = findBinary(entryPath);
      if (nested) {
        return nested;
      }
      continue;
    }
    if (entry.name === "dws" || entry.name === "dws.exe") {
      return entryPath;
    }
  }
  return "";
}

function extractArchive(archivePath, destDir) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "dws-npm-bin-"));
  try {
    if (archivePath.endsWith(".tar.gz")) {
      run("tar", ["-xzf", archivePath, "-C", tmpDir]);
    } else if (process.platform === "win32") {
      run("powershell.exe", [
        "-NoLogo",
        "-NoProfile",
        "-Command",
        `Expand-Archive -Path '${archivePath.replace(/'/g, "''")}' -DestinationPath '${tmpDir.replace(/'/g, "''")}' -Force`,
      ]);
    } else {
      run("unzip", ["-q", archivePath, "-d", tmpDir]);
    }

    const binaryPath = findBinary(tmpDir);
    if (!binaryPath) {
      throw new Error(`dws binary not found in archive ${archivePath}`);
    }

    ensureCleanDir(destDir);
    const targetName = process.platform === "win32" ? "dws.exe" : "dws";
    const targetPath = path.join(destDir, targetName);
    fs.copyFileSync(binaryPath, targetPath);
    if (process.platform !== "win32") {
      fs.chmodSync(targetPath, 0o755);
    }
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

function extractSkills(zipPath, destDir) {
  ensureCleanDir(destDir);
  if (process.platform === "win32") {
    run("powershell.exe", [
      "-NoLogo",
      "-NoProfile",
      "-Command",
      `Expand-Archive -Path '${zipPath.replace(/'/g, "''")}' -DestinationPath '${destDir.replace(/'/g, "''")}' -Force`,
    ]);
    return;
  }
  run("unzip", ["-q", zipPath, "-d", destDir]);
}

function copyChildren(srcDir, destDir) {
  fs.mkdirSync(destDir, { recursive: true });
  for (const entry of fs.readdirSync(srcDir)) {
    fs.cpSync(path.join(srcDir, entry), path.join(destDir, entry), { recursive: true, force: true });
  }
}

function installSkillsToHomes(skillRoot) {
  const homeDir = os.homedir();
  let installed = 0;

  AGENT_DIRS.forEach((agentDir, index) => {
    const baseDir = path.join(homeDir, agentDir);
    const parentGate = path.dirname(baseDir);
    if (index > 0 && !fs.existsSync(parentGate)) {
      return;
    }
    // Mutual exclusion: back up + remove multi leftovers before laying down
    // mono. Directories only — a stray file named dingtalk-x.md must survive.
    // Non-interactive installs cannot confirm, so removals stay reversible via
    // ~/.dws/skill-backups/ (backup failure keeps the dir).
    if (fs.existsSync(baseDir)) {
      for (const entry of fs.readdirSync(baseDir, { withFileTypes: true })) {
        if (entry.isDirectory() && (entry.name.startsWith("dingtalk-") || entry.name === "dws-shared")) {
          backupAndRemoveSkillDir(homeDir, path.join(baseDir, entry.name));
        }
      }
    }
    const destDir = path.join(baseDir, "dws");
    if (!backupAndRemoveSkillDir(homeDir, destDir)) {
      // Refreshing an existing skill: on backup failure keep the user's
      // copy and skip this target.
      console.warn(`⚠️  跳过 ${destDir}（保留原目录）`);
      installed += 1;
      return;
    }
    copyChildren(skillRoot, destDir);
    installed += 1;
  });

  if (installed === 0) {
    copyChildren(skillRoot, path.join(homeDir, ".agents", "skills", "dws"));
  }
}

// multiTreeHasSkills mirrors multi_tree_has_skills in scripts/install.sh and
// Test-MultiTreeHasSkills in scripts/install.ps1: true only when the multi
// bundle carries at least one product skill (a subdir with SKILL.md). An
// empty or corrupt multi/ tree must never select the multi branch nor refresh
// the multi cache — installing it would wipe existing skills and lay down
// nothing.
function multiTreeHasSkills(dir) {
  if (!fs.existsSync(dir) || !fs.statSync(dir).isDirectory()) {
    return false;
  }
  return fs
    .readdirSync(dir, { withFileTypes: true })
    .some((e) => e.isDirectory() && fs.existsSync(path.join(dir, e.name, "SKILL.md")));
}

// installMultiSkillsToHomes mirrors installSkillsToHomes for the multi bundle:
// every product skill becomes a sibling directory of the agent home. Mutual
// exclusion: the mono leftover (dws/) and stale dingtalk-* skills not present
// in the new bundle are removed first.
function installMultiSkillsToHomes(multiRoot) {
  const homeDir = os.homedir();
  const skills = fs
    .readdirSync(multiRoot, { withFileTypes: true })
    .filter((e) => e.isDirectory() && fs.existsSync(path.join(multiRoot, e.name, "SKILL.md")))
    .map((e) => e.name);
  if (skills.length === 0) {
    throw new Error(`no product skills found under ${multiRoot}`);
  }
  const skillSet = new Set(skills);
  let installed = 0;

  const installToBase = (baseDir) => {
    fs.mkdirSync(baseDir, { recursive: true });
    // Mutual exclusion: back up + remove the mono leftover, then stale multi
    // skills (dingtalk-* or dws-shared) not in the new bundle.
    backupAndRemoveSkillDir(homeDir, path.join(baseDir, "dws"));
    for (const entry of fs.readdirSync(baseDir, { withFileTypes: true })) {
      if (
        entry.isDirectory() &&
        (entry.name.startsWith("dingtalk-") || entry.name === "dws-shared") &&
        !skillSet.has(entry.name)
      ) {
        backupAndRemoveSkillDir(homeDir, path.join(baseDir, entry.name));
      }
    }
    for (const name of skills) {
      const destDir = path.join(baseDir, name);
      if (!backupAndRemoveSkillDir(homeDir, destDir)) {
        // Refreshing an existing skill: on backup failure keep the user's
        // copy and skip this skill.
        console.warn(`⚠️  跳过 ${destDir}（保留原目录）`);
        continue;
      }
      copyChildren(path.join(multiRoot, name), destDir);
    }
  };

  AGENT_DIRS.forEach((agentDir, index) => {
    const baseDir = path.join(homeDir, agentDir);
    const parentGate = path.dirname(baseDir);
    if (index > 0 && !fs.existsSync(parentGate)) {
      return;
    }
    installToBase(baseDir);
    installed += 1;
  });

  if (installed === 0) {
    installToBase(path.join(homeDir, ".agents", "skills"));
  }
}

// resolveSkillMode mirrors scripts/install.sh: DWS_SKILL_MODE (mono|multi)
// wins; multi is the default. The --skill-mode flag accepts both the space
// form (`--skill-mode mono`) and the equals form (`--skill-mode=mono`).
function resolveSkillMode() {
  const raw = (process.env.DWS_SKILL_MODE || "").trim().toLowerCase();
  if (raw === "mono" || raw === "multi") {
    return raw;
  }
  if (raw !== "") {
    throw new Error(`invalid DWS_SKILL_MODE='${process.env.DWS_SKILL_MODE}'. Use 'mono' or 'multi'.`);
  }
  let fromFlag;
  const flagIndex = process.argv.indexOf("--skill-mode");
  if (flagIndex !== -1 && process.argv[flagIndex + 1]) {
    fromFlag = process.argv[flagIndex + 1];
  } else {
    const equalsArg = process.argv.find((arg) => arg.startsWith("--skill-mode="));
    if (equalsArg) {
      fromFlag = equalsArg.slice("--skill-mode=".length);
    }
  }
  if (fromFlag !== undefined) {
    const mode = fromFlag.trim().toLowerCase();
    if (mode === "mono" || mode === "multi") {
      return mode;
    }
    throw new Error(`invalid --skill-mode '${fromFlag}'. Use 'mono' or 'multi'.`);
  }
  return "multi";
}

// cacheUserSkills copies the mono and multi trees out of the freshly extracted
// dws-skills.zip into ~/.dws/skills/{mono,multi}/ so that `dws skill setup`
// can fall back to a user-local cache when --source is not provided. A cache
// is only refreshed when the new bundle actually carries that tree — an
// empty/corrupt multi/ (or a missing mono tree) must never wipe a previously
// good cache.
function cacheUserSkills(extractedSkillsRoot) {
  const cacheBase = path.join(os.homedir(), ".dws", "skills");

  const monoSource = fs.existsSync(path.join(extractedSkillsRoot, "mono", "SKILL.md"))
    ? path.join(extractedSkillsRoot, "mono")
    : extractedSkillsRoot;
  if (fs.existsSync(path.join(monoSource, "SKILL.md"))) {
    const monoCache = path.join(cacheBase, "mono");
    fs.rmSync(monoCache, { recursive: true, force: true });
    copyChildren(monoSource, monoCache);
  }

  const multiSource = path.join(extractedSkillsRoot, "multi");
  if (multiTreeHasSkills(multiSource)) {
    const multiCache = path.join(cacheBase, "multi");
    fs.rmSync(multiCache, { recursive: true, force: true });
    copyChildren(multiSource, multiCache);
  }
}

function main() {
  const packageRoot = __dirname;
  const assetsDir = path.join(packageRoot, "assets");
  const vendorDir = path.join(packageRoot, "vendor");
  // Extract dws-skills.zip into a staging directory so we can split mono/
  // (installed to agent homes) from multi/ (cached for later setup use).
  const skillsStaging = path.join(packageRoot, "share", "skills");
  const assetName = PLATFORM_MAP[`${process.platform}-${process.arch}`];
  if (!assetName) {
    throw new Error(`unsupported platform: ${process.platform}/${process.arch}`);
  }

  const archivePath = path.join(assetsDir, assetName);
  const skillsPath = path.join(assetsDir, "dws-skills.zip");
  if (!fs.existsSync(archivePath)) {
    throw new Error(`missing platform archive: ${archivePath}`);
  }
  if (!fs.existsSync(skillsPath)) {
    throw new Error(`missing skills archive: ${skillsPath}`);
  }

  extractArchive(archivePath, vendorDir);
  extractSkills(skillsPath, skillsStaging);

  // For backward compatibility, the zip root carries a copy of mono content
  // (SKILL.md + references/ + scripts/). Prefer the explicit mono/ subdir
  // when present; fall back to the staging root otherwise.
  const monoRoot = fs.existsSync(path.join(skillsStaging, "mono", "SKILL.md"))
    ? path.join(skillsStaging, "mono")
    : skillsStaging;
  // A mono install requires an actual SKILL.md at the root of monoRoot. On a
  // multi-only zip monoRoot would degrade to the staging root and copy the
  // whole bundle (multi/ included) into a dws/ directory — skip instead.
  const monoHasSkill = fs.existsSync(path.join(monoRoot, "SKILL.md"));
  const multiRoot = path.join(skillsStaging, "multi");
  const skillMode = resolveSkillMode();
  if (skillMode === "multi" && multiTreeHasSkills(multiRoot)) {
    console.log(`Skill mode: multi — installing per-product skills`);
    installMultiSkillsToHomes(multiRoot);
  } else {
    if (skillMode === "multi") {
      console.log("multi skill tree not found or empty in bundle; falling back to mono.");
    }
    if (monoHasSkill) {
      installSkillsToHomes(monoRoot);
    } else {
      console.log("mono skill tree not found in bundle; skipping skill install.");
    }
  }
  cacheUserSkills(skillsStaging);
}

main();
