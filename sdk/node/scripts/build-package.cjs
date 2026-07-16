#!/usr/bin/env node
"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const SDK_ROOT = path.resolve(__dirname, "..");
const DIST_DIR = path.join(SDK_ROOT, "dist");
const PROTO_NAMES = [
  "alert.proto",
  "buspacket.proto",
  "handshake.proto",
  "heartbeat.proto",
  "job.proto",
  "policy.proto",
  "safety.proto",
];

function compileSource() {
  const tsc = path.join(SDK_ROOT, "node_modules", "typescript", "bin", "tsc");
  assert.equal(fs.existsSync(tsc), true, "run npm ci before building the package");
  const result = spawnSync(process.execPath, [tsc, "--project", "tsconfig.json"], {
    cwd: SDK_ROOT,
    encoding: "utf8",
    env: process.env,
  });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    const output = [result.stdout, result.stderr].filter(Boolean).join("\n");
    fs.rmSync(DIST_DIR, { recursive: true, force: true });
    throw new Error(`TypeScript build failed (${result.status})\n${output}`);
  }
}

function copyCanonicalProtos() {
  const sourceDir = path.resolve(SDK_ROOT, "..", "..", "proto", "cordum", "agent", "v1");
  const actual = fs
    .readdirSync(sourceDir, { withFileTypes: true })
    .filter((entry) => entry.isFile() && entry.name.endsWith(".proto"))
    .map((entry) => entry.name)
    .sort();
  assert.deepEqual(actual, [...PROTO_NAMES].sort(), "canonical CAP proto set changed");

  const destinationDir = path.join(DIST_DIR, "proto", "cordum", "agent", "v1");
  fs.mkdirSync(destinationDir, { recursive: true });
  for (const name of PROTO_NAMES) {
    fs.copyFileSync(path.join(sourceDir, name), path.join(destinationDir, name));
  }
}

function main() {
  fs.rmSync(DIST_DIR, { recursive: true, force: true });
  compileSource();
  copyCanonicalProtos();
}

try {
  main();
} catch (error) {
  console.error(error instanceof Error ? error.stack : error);
  process.exitCode = 1;
}
