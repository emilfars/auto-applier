#!/usr/bin/env node
// E-COV (plan-revamp.md section 5): share of changed, executable, non-test
// lines that a test executes. Usage: node scripts/changed-coverage.mjs [base-ref]
// Compares the working tree (including untracked files) against base-ref
// (default HEAD) and exits 1 below THRESHOLD. Backend needs TEST_DATABASE_URL
// for the Postgres-backed packages, same as verify.sh.
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, existsSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import assert from "node:assert/strict";

const THRESHOLD = 90;
const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const TS_PACKAGES = ["web", "extension", "packages/fill-mappings"];
const GO_MODULE = "github.com/auto-applier/backend/";

const isSource = (f) =>
  (/^backend\/.*\.go$/.test(f) && !f.endsWith("_test.go")) ||
  (TS_PACKAGES.some((p) => f.startsWith(`${p}/src/`)) &&
    /\.(ts|tsx)$/.test(f) &&
    !/\.test\.|test-setup|\.d\.ts$/.test(f));

// Map of file -> Set of added/modified line numbers, from a zero-context diff.
export function parseDiff(diff) {
  const changed = new Map();
  let file = null;
  for (const line of diff.split("\n")) {
    if (line.startsWith("+++ ")) {
      file = line.startsWith("+++ b/") ? line.slice(6) : null;
      continue;
    }
    const hunk = /^@@ -\S+ \+(\d+)(?:,(\d+))? @@/.exec(line);
    if (!hunk || !file) continue;
    const start = Number(hunk[1]);
    const count = hunk[2] === undefined ? 1 : Number(hunk[2]);
    if (!changed.has(file)) changed.set(file, new Set());
    for (let i = 0; i < count; i++) changed.get(file).add(start + i);
  }
  return changed;
}

// Both parsers return Map of file -> Map of line -> covered(boolean).
export function parseGoProfile(profile) {
  const lines = new Map();
  for (const row of profile.split("\n")) {
    const m = /^(.+):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$/.exec(row);
    if (!m) continue;
    const file = `backend/${m[1].replace(GO_MODULE, "")}`;
    if (!lines.has(file)) lines.set(file, new Map());
    const hit = Number(m[4]) > 0;
    for (let n = Number(m[2]); n <= Number(m[3]); n++) {
      lines.get(file).set(n, hit || lines.get(file).get(n) === true);
    }
  }
  return lines;
}

export function parseLcov(lcov, prefix) {
  const lines = new Map();
  let file = null;
  for (const row of lcov.split("\n")) {
    if (row.startsWith("SF:")) {
      file = `${prefix}/${row.slice(3)}`;
      lines.set(file, new Map());
    } else if (row.startsWith("DA:") && file) {
      const [n, hits] = row.slice(3).split(",").map(Number);
      lines.get(file).set(n, hits > 0);
    }
  }
  return lines;
}

export function score(changed, coverage) {
  let coverable = 0;
  let covered = 0;
  const misses = [];
  for (const [file, nums] of changed) {
    const fileCov = coverage.get(file);
    for (const n of nums) {
      if (!fileCov?.has(n)) continue;
      coverable++;
      if (fileCov.get(n)) covered++;
      else misses.push(`${file}:${n}`);
    }
  }
  return { coverable, covered, misses };
}

function selfTest() {
  const changed = parseDiff("+++ b/a.go\n@@ -1,0 +2,2 @@\n+x\n+y\n+++ /dev/null\n@@ -1 +0,0 @@\n");
  assert.deepEqual([...changed.get("a.go")], [2, 3]);
  const go = parseGoProfile(`mode: set\n${GO_MODULE}a.go:2.1,2.9 1 1\n${GO_MODULE}a.go:3.1,3.9 1 0\n`);
  assert.equal(go.get("backend/a.go").get(2), true);
  assert.equal(go.get("backend/a.go").get(3), false);
  const lcov = parseLcov("SF:src/a.ts\nDA:2,1\nDA:3,0\nend_of_record\n", "web");
  assert.deepEqual(score(new Map([["web/src/a.ts", new Set([2, 3, 9])]]), lcov), {
    coverable: 2,
    covered: 1,
    misses: ["web/src/a.ts:3"],
  });
  console.log("changed-coverage self-test: ok");
}

function main() {
  const base = process.argv[2] ?? "HEAD";
  const git = (...args) => execFileSync("git", args, { cwd: ROOT, encoding: "utf8", maxBuffer: 1 << 28 });
  const untracked = git("ls-files", "--others", "--exclude-standard").split("\n").filter(Boolean);
  let diff = git("diff", "-U0", "--no-color", base, "--");
  for (const f of untracked) {
    const r = spawnSync("git", ["diff", "-U0", "--no-color", "--no-index", "/dev/null", f], { cwd: ROOT, encoding: "utf8" });
    diff += r.stdout;
  }
  const changed = new Map([...parseDiff(diff)].filter(([f]) => isSource(f)));
  if (changed.size === 0) {
    console.log("E-COV: no changed source lines (n/a)");
    return;
  }

  const tmp = mkdtempSync(join(tmpdir(), "aa-cov-"));
  const coverage = new Map();
  const run = (cmd, args, cwd) => {
    const r = spawnSync(cmd, args, { cwd, encoding: "utf8", maxBuffer: 1 << 28 });
    if (r.status !== 0) {
      console.error(r.stdout, r.stderr);
      throw new Error(`E-COV: ${cmd} ${args.join(" ")} failed in ${cwd}`);
    }
  };
  try {
    if ([...changed.keys()].some((f) => f.startsWith("backend/"))) {
      const profile = join(tmp, "go.out");
      run("go", ["test", "-count=1", "-p", "1", "-coverpkg=./...", `-coverprofile=${profile}`, "./..."], join(ROOT, "backend"));
      for (const [f, m] of parseGoProfile(readFileSync(profile, "utf8"))) coverage.set(f, m);
    }
    for (const pkg of TS_PACKAGES) {
      if (![...changed.keys()].some((f) => f.startsWith(`${pkg}/`))) continue;
      const dir = join(tmp, pkg.replaceAll("/", "_"));
      run(
        "npx",
        ["vitest", "run", "--coverage.enabled", "--coverage.provider=v8", "--coverage.all", "--coverage.include=src/**", "--coverage.reporter=lcovonly", `--coverage.reportsDirectory=${dir}`],
        join(ROOT, pkg),
      );
      const lcov = join(dir, "lcov.info");
      if (existsSync(lcov)) for (const [f, m] of parseLcov(readFileSync(lcov, "utf8"), pkg)) coverage.set(f, m);
    }
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }

  const { coverable, covered, misses } = score(changed, coverage);
  if (coverable === 0) {
    console.log("E-COV: changed lines are not executable (n/a)");
    return;
  }
  const pct = (100 * covered) / coverable;
  console.log(`E-COV: ${covered}/${coverable} changed executable lines covered = ${pct.toFixed(1)}% (threshold ${THRESHOLD}%)`);
  if (misses.length) console.log(`uncovered:\n  ${misses.join("\n  ")}`);
  if (pct < THRESHOLD) process.exit(1);
}

if (process.argv[2] === "--self-test") selfTest();
else main();
