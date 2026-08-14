#!/usr/bin/env node
/**
 * install_js_smoke.mjs — smoke test for build/npm/install.js (npm postinstall).
 *
 * Runs the REAL build/npm/install.js against a staged fake package:
 *
 *   <tmp>/pkg/
 *     install.js                 (copied from build/npm/install.js)
 *     assets/
 *       dws-<os>-<arch>.tar.gz   (dummy archive holding a fake `dws` binary)
 *       dws-skills.zip           (tiny release-layout fixture built on the fly,
 *                                 NOT the real skills/ tree)
 *
 * Scenarios (each with an isolated fake HOME):
 *   1. multi install        — dingtalk-* and dws-shared land as sibling
 *                             skills, mono leftover dws/ and stale
 *                             dingtalk-* are removed, and the
 *                             ~/.dws/skills/{multi,mono} caches fill.
 *   2. empty multi/ tree    — warns and falls back to mono instead of
 *                             crashing postinstall; a previously good multi
 *                             cache is NOT wiped.
 *   3. bogus mode           — DWS_SKILL_MODE=bogus exits non-zero with an
 *                             "invalid DWS_SKILL_MODE" error.
 *   4. multi-only zip, mono — mono install is skipped with a warning; the
 *                             staging root is NOT copied into a dws/ dir.
 *   5. multi backup failure — preserves mono, writes no multi skill, and
 *                             reports postinstall failure.
 *   6. mono backup failure  — preserves multi, writes no mono skill, and
 *                             reports postinstall failure.
 *   7. mono switch          — migrates only centrally owned multi Skills.
 *   8. cache copy failure   — preserves the previous complete cache.
 *   9. multi publish failure — restores the complete previous Skill set.
 *  10. multi backup failure  — restores every earlier successful backup.
 *  11. mono transaction failure — restores every managed multi Skill after
 *                                 later backup or mono publish failure.
 *
 * Requirements: unix host with tar/zip/unzip on PATH (the same tools
 * install.js itself shells out to). Skips cleanly on win32.
 *
 * Usage (standalone; there is intentionally no Go test harness for the npm
 * installer — test/scripts/install_script_test.go only execs POSIX sh):
 *
 *   node test/scripts/install_js_smoke.mjs        # self-contained, <10s
 */

import assert from "node:assert/strict";
import childProcess from "node:child_process";
import fs from "node:fs";
import { createRequire } from "node:module";
import os from "node:os";
import path from "node:path";
import url from "node:url";

const PLATFORM_MAP = {
  "darwin-x64": "dws-darwin-amd64.tar.gz",
  "darwin-arm64": "dws-darwin-arm64.tar.gz",
  "linux-x64": "dws-linux-amd64.tar.gz",
  "linux-arm64": "dws-linux-arm64.tar.gz",
};

const repoRoot = path.resolve(path.dirname(url.fileURLToPath(import.meta.url)), "..", "..");
const installJsSource = path.join(repoRoot, "build", "npm", "install.js");
const require = createRequire(import.meta.url);
const {
  UPSTREAM_AGENTS,
  resolvedAgentTargets,
  agentTargetDetected,
  backupAndRemoveSkillDir,
  movePathRecoverablySync,
  publishCacheAtomically,
  publishCanonicalLinksAtomically,
  publishManagedMonoSkillSetAtomically,
  publishManagedMultiSkillSetAtomically,
} = require(installJsSource);

assert.equal(UPSTREAM_AGENTS.length, 76, "the complete upstream Agent registry is pinned");
assert.equal(new Set(UPSTREAM_AGENTS.map(({ id }) => id)).size, 76, "upstream Agent IDs are unique");
assert.equal(UPSTREAM_AGENTS.filter(({ universal }) => universal).length, 19, "upstream universal classification is pinned");
assert.equal(UPSTREAM_AGENTS.filter(({ universal }) => !universal).length, 57, "upstream non-universal classification is pinned");
assert.equal(UPSTREAM_AGENTS.filter(({ agentDir }) => agentDir === null).length, 2, "no-global Agents are retained in the registry");
assert.equal(UPSTREAM_AGENTS.filter(({ agentDir }) => agentDir === ".agents/skills").length, 6, "canonical-direct Agents need no target");
assert.equal(resolvedAgentTargets(path.join(os.tmpdir(), "dws-registry-home")).length, 70, "65 upstream roots, qoderwork, and 4 migration roots are deduplicated");
const detectionHome = fs.mkdtempSync(path.join(os.tmpdir(), "dws-agent-detect-"));
fs.mkdirSync(path.join(detectionHome, ".config", "kimchi"), { recursive: true });
fs.mkdirSync(path.join(detectionHome, ".tabnine"), { recursive: true });
const detectionTargets = resolvedAgentTargets(detectionHome);
for (const id of ["kimchi", "tabnine-cli"]) {
  assert.equal(agentTargetDetected(detectionTargets.find((target) => target.id === id)), true, `${id} shallow install marker is detected`);
}
fs.rmSync(detectionHome, { recursive: true, force: true });
const assetName = PLATFORM_MAP[`${process.platform}-${process.arch}`];

if (process.platform === "win32" || !assetName) {
  console.log(`SKIP: install.js smoke test needs a unix host with tar/zip/unzip (got ${process.platform}-${process.arch})`);
  process.exit(0);
}
for (const tool of ["tar", "zip", "unzip"]) {
  try {
    childProcess.execFileSync("sh", ["-c", `command -v ${tool}`], { stdio: "ignore" });
  } catch {
    console.log(`SKIP: required tool not on PATH: ${tool}`);
    process.exit(0);
  }
}

function sh(command, args, options = {}) {
  childProcess.execFileSync(command, args, { stdio: "ignore", ...options });
}

function writeFile(filePath, content, mode = 0o644) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, content, { mode });
}

function writeManagedState(home, names) {
  const state = {
    version: "old",
    official_skills: names,
    updated_skills: names,
    managed_skills: names.map((name) => ({
      name,
      version: "old",
      source: "test",
      digest: `sha256:${"0".repeat(64)}`,
      digest_scope: "skill-directory-v1",
    })),
    updated_at: "2026-01-01T00:00:00Z",
  };
  writeFile(path.join(home, ".dws", "skills-state.json"), `${JSON.stringify(state, null, 2)}\n`);
}

function crossDeviceError() {
  return Object.assign(new Error("injected cross-device rename"), { code: "EXDEV" });
}

function runCrossDeviceMoveContract() {
  const roots = [];
  const fixture = (name) => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), `dws-installjs-exdev-${name}-`));
    roots.push(root);
    const src = path.join(root, "external", "skill");
    const dest = path.join(root, "home", ".dws", "skill-backups", "stamp", "skill");
    writeFile(path.join(src, "SKILL.md"), "old skill\n", 0o640);
    return { root, src, dest };
  };
  const injectedRename = (source, target) => {
    if (path.basename(source) === "skill" && path.basename(target) === "skill") {
      throw crossDeviceError();
    }
    fs.renameSync(source, target);
  };
  try {
    {
      const { src, dest } = fixture("success");
      fs.symlinkSync("SKILL.md", path.join(src, "skill-link"));
      fs.symlinkSync("missing.md", path.join(src, "dangling-link"));
      movePathRecoverablySync(src, dest, { renameFn: injectedRename });
      assert.equal(fs.lstatSync(dest).isDirectory(), true);
      assert.equal(fs.statSync(path.join(dest, "SKILL.md")).mode & 0o777, 0o640);
      assert.equal(fs.lstatSync(path.join(dest, "skill-link")).isSymbolicLink(), true);
      assert.equal(fs.readlinkSync(path.join(dest, "skill-link")), "SKILL.md");
      assert.equal(fs.lstatSync(path.join(dest, "dangling-link")).isSymbolicLink(), true);
      assert.equal(fs.readlinkSync(path.join(dest, "dangling-link")), "missing.md");
      assert.equal(fs.existsSync(src), false, "verified EXDEV move removes source last");

      const restored = path.join(path.dirname(src), "restored-skill");
      movePathRecoverablySync(dest, restored, {
        renameFn(source, target) {
          if (source === dest && target === restored) throw crossDeviceError();
          fs.renameSync(source, target);
        },
      });
      assert.equal(fs.readFileSync(path.join(restored, "SKILL.md"), "utf8"), "old skill\n");
      assert.equal(fs.existsSync(dest), false, "EXDEV rollback consumes verified backup");
    }

    for (const [name, options] of [
      ["copy", { copyFn() { throw new Error("copy failed"); } }],
      ["verify", { verifyFn() { throw new Error("verify failed"); } }],
    ]) {
      const { root, src, dest } = fixture(name);
      assert.throws(
        () => movePathRecoverablySync(src, dest, { renameFn: injectedRename, ...options }),
        new RegExp(`${name} failed`),
      );
      assert.equal(fs.readFileSync(path.join(src, "SKILL.md"), "utf8"), "old skill\n");
      assert.equal(fs.existsSync(dest), false);
      const backupParent = path.dirname(dest);
      if (fs.existsSync(backupParent)) {
        assert.equal(
          fs.readdirSync(backupParent).some((entry) => entry.startsWith(".skill.cross-device-")),
          false,
          `${name} failure cleans destination-filesystem staging`,
        );
      }
      assert.ok(root);
    }

    {
      const { src, dest } = fixture("remove");
      assert.throws(
        () => movePathRecoverablySync(src, dest, {
          renameFn: injectedRename,
          removeFn(target) {
            if (target === src) throw new Error("remove failed");
            fs.rmSync(target, { recursive: true, force: true });
          },
        }),
        /both copies retained .*remove failed/,
      );
      assert.equal(fs.readFileSync(path.join(src, "SKILL.md"), "utf8"), "old skill\n");
      assert.equal(fs.readFileSync(path.join(dest, "SKILL.md"), "utf8"), "old skill\n");
    }

    {
      const { src, dest } = fixture("read-only");
      fs.chmodSync(src, 0o555);
      movePathRecoverablySync(src, dest, { renameFn: injectedRename });
      assert.equal(fs.statSync(dest).mode & 0o777, 0o555, "read-only directory mode is restored after publish");
      assert.equal(fs.readFileSync(path.join(dest, "SKILL.md"), "utf8"), "old skill\n");
      fs.chmodSync(dest, 0o755);
    }

    {
      const { src, dest } = fixture("read-only-remove-failure");
      fs.chmodSync(src, 0o555);
      assert.throws(
        () => movePathRecoverablySync(src, dest, {
          renameFn: injectedRename,
          removeFn(target) {
            if (target === src) throw new Error("remove failed");
            fs.rmSync(target, { recursive: true, force: true });
          },
        }),
        /both copies retained .*remove failed/,
      );
      assert.equal(fs.statSync(src).mode & 0o777, 0o555, "failed removal restores source directory mode");
      fs.chmodSync(src, 0o755);
      fs.chmodSync(dest, 0o755);
    }

    {
      const { src, dest } = fixture("permission");
      let copied = false;
      const permission = Object.assign(new Error("permission denied"), { code: "EACCES" });
      assert.throws(
        () => movePathRecoverablySync(src, dest, {
          renameFn() { throw permission; },
          copyFn() { copied = true; },
        }),
        /permission denied/,
      );
      assert.equal(copied, false, "EACCES must not be mistaken for EXDEV");
      assert.equal(fs.readFileSync(path.join(src, "SKILL.md"), "utf8"), "old skill\n");
    }

    {
      const root = fs.mkdtempSync(path.join(os.tmpdir(), "dws-installjs-dangling-collision-"));
      roots.push(root);
      const home = path.join(root, "home");
      const victim = path.join(home, ".agents", "skills", "dingtalk-chat");
      writeFile(path.join(victim, "SKILL.md"), "old\n");
      const stamp = "20260813-150000";
      const occupied = path.join(home, ".dws", "skill-backups", stamp, ".agents-skills-dingtalk-chat");
      fs.mkdirSync(path.dirname(occupied), { recursive: true });
      fs.symlinkSync("missing-backup", occupied);
      const backup = backupAndRemoveSkillDir(home, victim, null, { backupStampFn: () => stamp });
      assert.ok(backup.includes(`${stamp}-1`), "dangling backup target selects a numbered sibling");
      assert.equal(fs.readlinkSync(occupied), "missing-backup", "dangling collision is never overwritten");
    }

    {
      const root = fs.mkdtempSync(path.join(os.tmpdir(), "dws-installjs-link-rollback-"));
      roots.push(root);
      const home = path.join(root, "home");
      const canonical = path.join(home, ".agents", "skills");
      const base = path.join(home, ".cursor", "skills");
      const canonicalSkill = path.join(canonical, "dingtalk-chat");
      const victim = path.join(base, "dingtalk-chat");
      writeFile(path.join(canonicalSkill, "SKILL.md"), "new\n");
      writeFile(path.join(victim, "SKILL.md"), "old\n");
      assert.throws(
        () => publishCanonicalLinksAtomically(home, canonical, base, ["dingtalk-chat"], [victim], {
          renameFn(source, target) {
            if (source === victim || (source.includes(`${path.sep}skill-backups${path.sep}`) && target === victim)) {
              throw crossDeviceError();
            }
            fs.renameSync(source, target);
          },
		  publishLinkFn() { throw new Error("link publish failed"); },
        }),
        /link publish failed/,
      );
      assert.equal(fs.readFileSync(path.join(victim, "SKILL.md"), "utf8"), "old\n");
    }
  } finally {
    for (const root of roots) fs.rmSync(root, { recursive: true, force: true });
  }
}

// stagePkg builds a fake npm package whose assets/dws-skills.zip contains
// exactly the given zip entries ({ "relative/path": "content" }) plus any
// listed empty directories. Returns { tmp, pkg, home }.
function stagePkg(zipEntries, emptyDirs = []) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "dws-installjs-smoke-"));
  const pkg = path.join(tmp, "pkg");
  const assets = path.join(pkg, "assets");
  fs.mkdirSync(assets, { recursive: true });
  fs.copyFileSync(installJsSource, path.join(pkg, "install.js"));

  const binStage = path.join(tmp, "bin-stage");
  writeFile(path.join(binStage, "dws"), "#!/bin/sh\necho fake-dws\n", 0o755);
  sh("tar", ["-czf", path.join(assets, assetName), "-C", binStage, "."]);

  const zipStage = path.join(tmp, "zip-stage");
  for (const [rel, content] of Object.entries(zipEntries)) {
    writeFile(path.join(zipStage, rel), content);
  }
  for (const dir of emptyDirs) {
    fs.mkdirSync(path.join(zipStage, dir), { recursive: true });
  }
  sh("zip", ["-qr", path.join(assets, "dws-skills.zip"), "."], { cwd: zipStage });

  return { tmp, pkg, home: path.join(tmp, "home") };
}

function runInstall(pkg, home, skillMode, extraEnv = {}) {
  const env = { ...process.env, HOME: home, ...extraEnv };
  if (skillMode !== undefined) {
    env.DWS_SKILL_MODE = skillMode;
  } else {
    delete env.DWS_SKILL_MODE;
  }
  return childProcess.spawnSync(process.execPath, [path.join(pkg, "install.js")], {
    env,
    encoding: "utf8",
  });
}

const scenarios = [];
function scenario(name, fn) {
  scenarios.push([name, fn]);
}

scenario("multi install lays out sibling skills and caches", () => {
  const { tmp, pkg, home } = stagePkg({
    "SKILL.md": "# mono root copy\n",
    "mono/SKILL.md": "# mono fixture\n",
    "multi/dingtalk-test/SKILL.md": "# dingtalk-test\n",
    "multi/dingtalk-test/references/guide.md": "guide\n",
    "multi/dws-shared/SKILL.md": "# dws-shared\n",
  });
  try {
    // Pre-existing state the multi install must reconcile.
    writeFile(path.join(home, ".agents", "skills", "dws", "SKILL.md"), "old mono\n");
    writeFile(path.join(home, ".agents", "skills", "dingtalk-stale", "SKILL.md"), "stale\n");
    writeManagedState(home, ["dingtalk-stale"]);
    writeFile(path.join(home, ".agents", "skills", "dingtalk-custom", "SKILL.md"), "market skill\n");
    writeFile(path.join(home, ".agents", "skills", "other-skill", "SKILL.md"), "not dws\n");

    const res = runInstall(pkg, home, undefined); // default mode = multi
    assert.equal(res.status, 0, `exit=${res.status}\nstdout=${res.stdout}\nstderr=${res.stderr}`);
    assert.match(res.stdout, /Skill mode: multi/);

    const base = path.join(home, ".agents", "skills");
    assert.ok(fs.existsSync(path.join(base, "dingtalk-test", "SKILL.md")), "dingtalk-test installed");
    assert.ok(fs.existsSync(path.join(base, "dingtalk-test", "references", "guide.md")), "references copied");
    assert.ok(fs.existsSync(path.join(base, "dws-shared", "SKILL.md")), "dws-shared installed");
    assert.ok(!fs.existsSync(path.join(base, "dws")), "mono leftover removed");
    assert.ok(!fs.existsSync(path.join(base, "dingtalk-stale")), "stale skill removed");
    assert.equal(fs.readFileSync(path.join(base, "dingtalk-custom", "SKILL.md"), "utf8"), "market skill\n", "unregistered dingtalk-* skill preserved");
    const state = JSON.parse(fs.readFileSync(path.join(home, ".dws", "skills-state.json"), "utf8"));
    const provenance = state.managed_skills.find((record) => record.name === "dingtalk-test");
    assert.equal(provenance.source, "npm-postinstall");
    assert.match(provenance.digest, /^sha256:[0-9a-f]{64}$/);
    assert.equal(provenance.digest_scope, "skill-directory-v1");
    assert.ok(fs.existsSync(path.join(base, "other-skill", "SKILL.md")), "non-DWS skill preserved");

    assert.ok(fs.existsSync(path.join(home, ".dws", "skills", "multi", "dingtalk-test", "SKILL.md")), "multi cache filled");
    assert.equal(fs.readFileSync(path.join(home, ".dws", "skills", "mono", "SKILL.md"), "utf8"), "# mono fixture\n", "mono cache from mono/ tree");
    assert.ok(fs.existsSync(path.join(pkg, "vendor", "dws")), "binary installed into vendor/");
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("pinned universal global topology installs canonical and retires private copies", () => {
  const { tmp, pkg, home } = stagePkg({
    "mono/SKILL.md": "# mono fixture\n",
    "multi/dingtalk-chat/SKILL.md": "# dingtalk-chat\n",
    "multi/dws-shared/SKILL.md": "# dws-shared\n",
  });
  try {
    // The pinned upstream installer treats global universal Agents as
    // canonical-only even when their registry also retains a distinct
    // globalSkillsDir for legacy discovery/removal. Exercise every unique
    // non-canonical universal root so this distinction cannot drift again.
    const retiredUniversalRoots = [
      ".config/agents/skills",
      ".gemini/antigravity/skills",
      ".gemini/antigravity-cli/skills",
      ".codex/skills",
      ".cursor/skills",
      ".deepagents/agent/skills",
      ".firebender/skills",
      ".gemini/skills",
      ".copilot/skills",
      ".config/opencode/skills",
    ];
    for (const root of retiredUniversalRoots) {
      writeFile(path.join(home, root, "dingtalk-chat", "SKILL.md"), `beta.6 copy in ${root}\n`);
    }
    writeFile(
      path.join(home, ".agents", "skills", "dws", "multi", "dingtalk-chat", "SKILL.md"),
      "old nested duplicate\n",
    );

    const res = runInstall(pkg, home, "multi");
    assert.equal(res.status, 0, `exit=${res.status}\nstdout=${res.stdout}\nstderr=${res.stderr}`);
    assert.ok(
      fs.existsSync(path.join(home, ".agents", "skills", "dingtalk-chat", "SKILL.md")),
      "universal canonical Skill installed",
    );
    for (const root of retiredUniversalRoots) {
      assert.ok(
        !fs.existsSync(path.join(home, root, "dingtalk-chat")),
        `beta.6 universal duplicate migrated from ${root}`,
      );
    }
    assert.ok(
      !fs.existsSync(path.join(home, ".agents", "skills", "dws")),
      "legacy nested duplicate retired",
    );
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("ZCode links its Agent root to the canonical store", () => {
  const { tmp, pkg, home } = stagePkg({
    "mono/SKILL.md": "# mono fixture\n",
    "multi/dingtalk-chat/SKILL.md": "# dingtalk-chat\n",
  });
  try {
    writeFile(path.join(home, ".zcode", "v2", "config.json"), "{}\n");
    writeFile(
      path.join(home, ".agents", "skills", "dws", "multi", "dingtalk-chat", "SKILL.md"),
      "old nested duplicate\n",
    );

    const res = runInstall(pkg, home, "multi");
    assert.equal(res.status, 0, `exit=${res.status}\nstdout=${res.stdout}\nstderr=${res.stderr}`);
    assert.ok(
      fs.existsSync(path.join(home, ".zcode", "skills", "dingtalk-chat", "SKILL.md")),
      "ZCode Skill resolves through canonical link",
    );
    assert.ok(fs.lstatSync(path.join(home, ".zcode", "skills", "dingtalk-chat")).isSymbolicLink());
    assert.ok(fs.existsSync(path.join(home, ".agents", "skills", "dingtalk-chat", "SKILL.md")));
    assert.ok(
      !fs.existsSync(path.join(home, ".agents", "skills", "dws")),
      "legacy generic duplicate retired",
    );
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("upstream Agent roots honor XDG and custom homes with relative links", () => {
  const { tmp, pkg, home } = stagePkg({
    "mono/SKILL.md": "# mono fixture\n",
    "multi/dingtalk-chat/SKILL.md": "# dingtalk-chat\n",
  });
  const xdg = path.join(tmp, "xdg config");
  const autohand = path.join(tmp, "autohand home");
  const claude = path.join(tmp, "claude config");
  const codex = path.join(tmp, "codex home");
  const hermes = path.join(tmp, "hermes home");
  try {
    writeFile(path.join(xdg, "goose", "config.yaml"), "enabled: true\n");
    writeFile(path.join(xdg, "agents", "detection.marker"), "amp\n");
    writeFile(path.join(xdg, "agents", "skills", "dingtalk-chat", "SKILL.md"), "old amp copy\n");
    writeFile(path.join(autohand, "config.json"), "{}\n");
    writeFile(path.join(claude, "config.json"), "{}\n");
    writeFile(path.join(claude, "skills", "dingtalk-chat", "SKILL.md"), "old claude copy\n");
    writeFile(path.join(codex, "config.toml"), "model=test\n");
    writeFile(path.join(codex, "skills", "dingtalk-chat", "SKILL.md"), "old codex copy\n");
    writeFile(path.join(hermes, "config.json"), "{}\n");
    writeFile(path.join(hermes, "skills", "dingtalk-chat", "SKILL.md"), "old hermes copy\n");
    writeFile(path.join(home, ".qoderwork", "config.json"), "{}\n");
    writeFile(path.join(home, ".amp", "skills", "dingtalk-chat", "SKILL.md"), "old DWS path\n");

    const res = runInstall(pkg, home, "multi", {
      XDG_CONFIG_HOME: xdg,
      AUTOHAND_HOME: autohand,
      CLAUDE_CONFIG_DIR: claude,
      CODEX_HOME: codex,
      HERMES_HOME: hermes,
    });
    assert.equal(res.status, 0, `exit=${res.status}\nstdout=${res.stdout}\nstderr=${res.stderr}`);

    for (const linked of [
      path.join(xdg, "goose", "skills", "dingtalk-chat"),
      path.join(autohand, "skills", "dingtalk-chat"),
      path.join(claude, "skills", "dingtalk-chat"),
      path.join(hermes, "skills", "dingtalk-chat"),
      path.join(home, ".qoderwork", "skills", "dingtalk-chat"),
    ]) {
      assert.ok(fs.lstatSync(linked).isSymbolicLink(), `${linked} is linked`);
      assert.ok(!path.isAbsolute(fs.readlinkSync(linked)), `${linked} uses a relative link`);
      assert.equal(fs.readFileSync(path.join(linked, "SKILL.md"), "utf8"), "# dingtalk-chat\n");
    }
    const backupText = [];
    const collectBackupText = (dir) => {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) collectBackupText(full);
        else if (entry.isFile()) backupText.push(fs.readFileSync(full, "utf8"));
      }
    };
    collectBackupText(path.join(home, ".dws", "skill-backups"));
    assert.ok(backupText.includes("old claude copy\n"), "CLAUDE_CONFIG_DIR content is recoverably backed up");
    assert.ok(backupText.includes("old hermes copy\n"), "HERMES_HOME content is recoverably backed up");
    for (const retired of [
      path.join(xdg, "agents", "skills", "dingtalk-chat"),
      path.join(codex, "skills", "dingtalk-chat"),
      path.join(home, ".amp", "skills", "dingtalk-chat"),
    ]) {
      assert.ok(!fs.existsSync(retired), `legacy/universal duplicate retired: ${retired}`);
    }
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("empty multi/ tree falls back to mono and keeps the old multi cache", () => {
  const { tmp, pkg, home } = stagePkg({
    "SKILL.md": "# mono root copy\n",
    "mono/SKILL.md": "# mono fixture\n",
    // Corrupt multi tree: a product subdir without SKILL.md.
    "multi/dingtalk-broken/references/guide.md": "orphan\n",
  });
  try {
    writeFile(path.join(home, ".agents", "skills", "dws", "SKILL.md"), "old mono\n");
    // A previously good multi cache must survive an empty/corrupt bundle.
    writeFile(path.join(home, ".dws", "skills", "multi", "dingtalk-good", "SKILL.md"), "good cache\n");

    const res = runInstall(pkg, home, "multi");
    assert.equal(res.status, 0, `exit=${res.status}\nstdout=${res.stdout}\nstderr=${res.stderr}`);
    assert.match(res.stdout, /falling back to mono/);

    const base = path.join(home, ".agents", "skills");
    assert.equal(fs.readFileSync(path.join(base, "dws", "SKILL.md"), "utf8"), "# mono fixture\n", "mono installed from mono/ tree");
    assert.ok(!fs.existsSync(path.join(base, "dingtalk-broken")), "broken multi skill not installed");
    assert.equal(
      fs.readFileSync(path.join(home, ".dws", "skills", "multi", "dingtalk-good", "SKILL.md"), "utf8"),
      "good cache\n",
      "previously good multi cache must not be wiped",
    );
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("bogus DWS_SKILL_MODE fails fast with a clear error", () => {
  const { tmp, pkg, home } = stagePkg({
    "mono/SKILL.md": "# mono fixture\n",
    "multi/dingtalk-test/SKILL.md": "# dingtalk-test\n",
  });
  try {
    const res = runInstall(pkg, home, "bogus");
    assert.notEqual(res.status, 0, "bogus mode must exit non-zero");
    assert.match(res.stderr, /invalid DWS_SKILL_MODE/);
    assert.ok(!fs.existsSync(path.join(home, ".agents", "skills", "dingtalk-test")), "nothing installed on mode error");
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("multi-only zip in mono mode skips skill install instead of copying staging root", () => {
  const { tmp, pkg, home } = stagePkg({
    // No root SKILL.md and no mono/ tree — a multi-only release layout.
    "multi/dingtalk-test/SKILL.md": "# dingtalk-test\n",
    "multi/dws-shared/SKILL.md": "# dws-shared\n",
  });
  try {
    const res = runInstall(pkg, home, "mono");
    assert.equal(res.status, 0, `exit=${res.status}\nstdout=${res.stdout}\nstderr=${res.stderr}`);
    assert.match(res.stdout, /mono skill tree not found.*skipping skill install/s);
    const base = path.join(home, ".agents", "skills");
    assert.ok(!fs.existsSync(path.join(base, "dws")), "staging root must not be copied into dws/");
    assert.ok(!fs.existsSync(path.join(home, ".dws", "skills", "mono")), "mono cache not refreshed without a mono tree");
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("multi backup failure preserves mono and reports failure", () => {
  const { tmp, pkg, home } = stagePkg({
    "mono/SKILL.md": "# mono fixture\n",
    "multi/dingtalk-test/SKILL.md": "# dingtalk-test\n",
    "multi/dws-shared/SKILL.md": "# dws-shared\n",
  });
  try {
    const base = path.join(home, ".agents", "skills");
    writeFile(path.join(base, "dws", "SKILL.md"), "old mono\n");
    // Poison the backup root: mkdirSync(<file>/<stamp>) must fail.
    writeFile(path.join(home, ".dws", "skill-backups"), "not a directory\n");

    const res = runInstall(pkg, home, "multi");
    assert.notEqual(res.status, 0, `exit=${res.status}\nstdout=${res.stdout}\nstderr=${res.stderr}`);
    assert.match(res.stderr, /未安装任何 multi Skill/);
    assert.equal(fs.readFileSync(path.join(base, "dws", "SKILL.md"), "utf8"), "old mono\n");
    assert.ok(!fs.existsSync(path.join(base, "dingtalk-test")), "product skill not installed after cleanup failure");
    assert.ok(!fs.existsSync(path.join(base, "dws-shared")), "shared skill not installed after cleanup failure");
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("mono backup failure preserves multi and reports failure", () => {
  const { tmp, pkg, home } = stagePkg({
    "mono/SKILL.md": "# mono fixture\n",
    "multi/dingtalk-test/SKILL.md": "# dingtalk-test\n",
  });
  try {
    const base = path.join(home, ".agents", "skills");
    writeFile(path.join(base, "dingtalk-test", "SKILL.md"), "old multi\n");
    writeManagedState(home, ["dingtalk-test"]);
    writeFile(path.join(home, ".dws", "skill-backups"), "not a directory\n");

    const res = runInstall(pkg, home, "mono");
    assert.notEqual(res.status, 0, `exit=${res.status}\nstdout=${res.stdout}\nstderr=${res.stderr}`);
    assert.match(res.stderr, /未安装任何 mono Skill/);
    assert.equal(fs.readFileSync(path.join(base, "dingtalk-test", "SKILL.md"), "utf8"), "old multi\n");
    assert.ok(!fs.existsSync(path.join(base, "dws")), "mono not installed after cleanup failure");
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("mono switch migrates exact pre-state official skills", () => {
  const { tmp, pkg, home } = stagePkg({
    "mono/SKILL.md": "# mono fixture\n",
    "multi/dingtalk-test/SKILL.md": "# dingtalk-test\n",
  });
  try {
    const base = path.join(home, ".agents", "skills");
    writeFile(path.join(base, "dingtalk-aitable", "SKILL.md"), "legacy official\n");
    writeFile(path.join(base, "dingtalk-custom", "SKILL.md"), "market skill\n");
    writeManagedState(home, ["dingtalk-aitable"]);

    const res = runInstall(pkg, home, "mono");
    assert.equal(res.status, 0, `exit=${res.status}\nstdout=${res.stdout}\nstderr=${res.stderr}`);
    assert.ok(!fs.existsSync(path.join(base, "dingtalk-aitable")), "pre-state official skill removed");
    assert.equal(fs.readFileSync(path.join(base, "dingtalk-custom", "SKILL.md"), "utf8"), "market skill\n");
    assert.ok(fs.existsSync(path.join(base, "dws", "SKILL.md")), "mono installed");
    assert.ok(!fs.existsSync(path.join(home, ".dws", "skills-state.json")), "mono clears centralized multi state");
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("cache copy failure preserves the previous complete cache", () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "dws-installjs-cache-"));
  const source = path.join(tmp, "source");
  const cache = path.join(tmp, "skills", "multi");
  try {
    writeFile(path.join(source, "dingtalk-new", "SKILL.md"), "new cache\n");
    writeFile(path.join(cache, "dingtalk-old", "SKILL.md"), "old cache\n");

    assert.throws(
      () =>
        publishCacheAtomically(source, cache, (_src, staged) => {
          writeFile(path.join(staged, "partial", "SKILL.md"), "partial\n");
          throw new Error("injected cache copy failure");
        }),
      /injected cache copy failure/,
    );
    assert.equal(fs.readFileSync(path.join(cache, "dingtalk-old", "SKILL.md"), "utf8"), "old cache\n");
    assert.ok(!fs.existsSync(path.join(cache, "dingtalk-new")), "failed refresh must not publish new cache");
    assert.ok(
      !fs.readdirSync(path.dirname(cache)).some((name) => name.startsWith(".multi.tmp-")),
      "failed refresh must clean its staging directory",
    );
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("canonical link publication never clobbers or deletes concurrent user data", () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "dws-installjs-canonical-race-"));
  try {
    {
      const home = path.join(tmp, "no-clobber-home");
      const canonical = path.join(home, ".agents", "skills");
      const base = path.join(home, ".cursor", "skills");
      const destination = path.join(base, "dingtalk-chat");
      writeFile(path.join(canonical, "dingtalk-chat", "SKILL.md"), "new\n");
      assert.throws(
        () => publishCanonicalLinksAtomically(home, canonical, base, ["dingtalk-chat"], [], {
          publishLinkFn(_source, target) {
            writeFile(path.join(target, "concurrent-user-data.txt"), "must survive\n");
            throw Object.assign(new Error("concurrent destination exists"), { code: "EEXIST" });
          },
        }),
        /concurrent destination exists/,
      );
      assert.equal(fs.readFileSync(path.join(destination, "concurrent-user-data.txt"), "utf8"), "must survive\n");
    }

    {
      const home = path.join(tmp, "rollback-home");
      const canonical = path.join(home, ".agents", "skills");
      const base = path.join(home, ".cursor", "skills");
      const first = path.join(base, "dingtalk-first");
      const second = path.join(base, "dingtalk-second");
      for (const name of ["dingtalk-first", "dingtalk-second"]) {
        writeFile(path.join(canonical, name, "SKILL.md"), `new ${name}\n`);
        writeFile(path.join(base, name, "SKILL.md"), `old ${name}\n`);
      }
      let publishCalls = 0;
      assert.throws(
        () => publishCanonicalLinksAtomically(
          home,
          canonical,
          base,
          ["dingtalk-first", "dingtalk-second"],
          [first, second],
          {
            publishLinkFn(source, target) {
              publishCalls += 1;
              if (publishCalls === 1) {
                const result = childProcess.spawnSync("ln", ["-P", source, target], { encoding: "utf8" });
                if (result.status !== 0) throw new Error(result.stderr || "ln -P failed");
                return;
              }
              fs.unlinkSync(first);
              writeFile(path.join(first, "concurrent-user-data.txt"), "must survive\n");
              throw new Error("injected second canonical publish failure");
            },
          },
        ),
        /concurrent object retained|rollback failed/,
      );
      assert.equal(fs.readFileSync(path.join(first, "SKILL.md"), "utf8"), "old dingtalk-first\n");
      assert.equal(fs.readFileSync(path.join(second, "SKILL.md"), "utf8"), "old dingtalk-second\n");
      const retained = fs.readdirSync(base)
        .filter((name) => name.startsWith(".dingtalk-first.rollback-"))
        .map((name) => path.join(base, name, "payload", "concurrent-user-data.txt"))
        .filter((candidate) => fs.existsSync(candidate));
      assert.equal(retained.length, 1, "concurrent replacement must be retained in quarantine");
      assert.equal(fs.readFileSync(retained[0], "utf8"), "must survive\n");
    }
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("multi set publish failure restores the complete previous set", () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "dws-installjs-multi-set-"));
  const home = path.join(tmp, "home");
  const source = path.join(tmp, "multi");
  const base = path.join(home, ".agents", "skills");
  const first = path.join(base, "dingtalk-first");
  const second = path.join(base, "dingtalk-second");
  try {
    writeFile(path.join(source, "dingtalk-first", "SKILL.md"), "new first\n");
    writeFile(path.join(source, "dingtalk-second", "SKILL.md"), "new second\n");
    writeFile(path.join(first, "SKILL.md"), "old first\n");
    writeFile(path.join(second, "SKILL.md"), "old second\n");

    const originalRename = fs.renameSync;
    assert.throws(
      () =>
        publishManagedMultiSkillSetAtomically(
          home,
          source,
          base,
          ["dingtalk-first", "dingtalk-second"],
          [first, second],
          {
            renameFn(src, dest) {
              if (
                src === first ||
                src === second ||
                (src.includes(`${path.sep}skill-backups${path.sep}`) && (dest === first || dest === second))
              ) {
                throw crossDeviceError();
              }
              if (src.includes(".dws-multi-set.tmp-") && path.basename(src) === "dingtalk-second") {
                throw new Error("injected second publish failure");
              }
              originalRename(src, dest);
            },
          },
        ),
      /injected second publish failure/,
    );
    assert.equal(fs.readFileSync(path.join(first, "SKILL.md"), "utf8"), "old first\n");
    assert.equal(fs.readFileSync(path.join(second, "SKILL.md"), "utf8"), "old second\n");
    assert.ok(
      !fs.readdirSync(base).some((name) => name.startsWith(".dws-multi-set.tmp-")),
      "failed publish must clean the multi-set staging directory",
    );
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

scenario("multi set backup failure restores earlier backups", () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "dws-installjs-multi-backup-"));
  const home = path.join(tmp, "home");
  const source = path.join(tmp, "multi");
  const base = path.join(home, ".agents", "skills");
  const first = path.join(base, "dingtalk-first");
  const second = path.join(base, "dingtalk-second");
  try {
    writeFile(path.join(source, "dingtalk-first", "SKILL.md"), "new first\n");
    writeFile(path.join(source, "dingtalk-second", "SKILL.md"), "new second\n");
    writeFile(path.join(first, "SKILL.md"), "old first\n");
    writeFile(path.join(second, "SKILL.md"), "old second\n");

    const originalRename = fs.renameSync;
    assert.throws(
      () =>
        publishManagedMultiSkillSetAtomically(
          home,
          source,
          base,
          ["dingtalk-first", "dingtalk-second"],
          [first, second],
          {
            renameFn(src, dest) {
              if (src === second) {
                throw new Error("injected second backup failure");
              }
              originalRename(src, dest);
            },
          },
        ),
      /failed to back up Skill directory/,
    );
    assert.equal(fs.readFileSync(path.join(first, "SKILL.md"), "utf8"), "old first\n");
    assert.equal(fs.readFileSync(path.join(second, "SKILL.md"), "utf8"), "old second\n");
    assert.ok(!fs.existsSync(path.join(base, "dingtalk-first", "new-only")));
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

for (const failureKind of ["backup", "publish"]) {
  scenario(`mono set ${failureKind} failure restores the complete previous set`, () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "dws-installjs-mono-set-"));
    const home = path.join(tmp, "home");
    const source = path.join(tmp, "mono");
    const base = path.join(home, ".agents", "skills");
    const first = path.join(base, "dingtalk-first");
    const second = path.join(base, "dingtalk-second");
    const dest = path.join(base, "dws");
    try {
      writeFile(path.join(source, "SKILL.md"), "new mono\n");
      writeFile(path.join(first, "SKILL.md"), "old first\n");
      writeFile(path.join(second, "SKILL.md"), "old second\n");

      const originalRename = fs.renameSync;
      assert.throws(
        () =>
          publishManagedMonoSkillSetAtomically(home, source, base, [dest, first, second], {
            renameFn(src, target) {
              if (failureKind === "backup" && src === second) {
                throw new Error("injected second backup failure");
              }
              if (
                failureKind === "publish" &&
                (src === first ||
                  src === second ||
                  (src.includes(`${path.sep}skill-backups${path.sep}`) && (target === first || target === second)))
              ) {
                throw crossDeviceError();
              }
              if (
                failureKind === "publish" &&
                src.includes(".dws-mono-set.tmp-") &&
                path.basename(src) === "dws"
              ) {
                throw new Error("injected mono publish failure");
              }
              originalRename(src, target);
            },
          }),
        /injected|failed to back up/,
      );
      assert.equal(fs.readFileSync(path.join(first, "SKILL.md"), "utf8"), "old first\n");
      assert.equal(fs.readFileSync(path.join(second, "SKILL.md"), "utf8"), "old second\n");
      assert.ok(!fs.existsSync(dest), "failed mono transaction must not expose dws/");
      assert.ok(
        !fs.readdirSync(base).some((name) => name.startsWith(".dws-mono-set.tmp-")),
        "failed mono transaction must clean its staging directory",
      );
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });
}

process.stdout.write("• EXDEV copy/verify/remove and transaction rollback contract ... ");
runCrossDeviceMoveContract();
process.stdout.write("ok\n");

for (const [name, fn] of scenarios) {
  process.stdout.write(`• ${name} ... `);
  fn();
  process.stdout.write("ok\n");
}
console.log(`OK — ${scenarios.length} install.js smoke scenarios passed`);
