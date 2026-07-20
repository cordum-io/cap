#!/usr/bin/env node
"use strict";

const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const SDK_ROOT = path.resolve(__dirname, "..");
const EXPECTED_PROTOS = [
  "alert.proto",
  "buspacket.proto",
  "handshake.proto",
  "heartbeat.proto",
  "job.proto",
  "policy.proto",
  "safety.proto",
].map((name) => `dist/proto/cordum/agent/v1/${name}`);

function run(command, args, cwd = SDK_ROOT) {
  const result = spawnSync(command, args, {
    cwd,
    encoding: "utf8",
    env: process.env,
    shell: false,
  });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    const output = [result.stdout, result.stderr].filter(Boolean).join("\n");
    throw new Error(`${command} ${args.join(" ")} failed (${result.status})\n${output}`);
  }
  return result.stdout.trim();
}

function runNpm(args, cwd = SDK_ROOT) {
  if (process.platform !== "win32") {
    return run("npm", args, cwd);
  }
  const npmCli =
    process.env.npm_execpath ??
    path.join(path.dirname(process.execPath), "node_modules", "npm", "bin", "npm-cli.js");
  assert.equal(fs.existsSync(npmCli), true, `cannot locate npm CLI: ${npmCli}`);
  return run(process.execPath, [npmCli, ...args], cwd);
}

function packSdk(tempDir) {
  fs.rmSync(path.join(SDK_ROOT, "dist"), { recursive: true, force: true });
  fs.rmSync(path.join(SDK_ROOT, ".build"), { recursive: true, force: true });
  runNpm(["run", "build"]);

  const packDir = path.join(tempDir, "package");
  fs.mkdirSync(packDir, { recursive: true });
  const output = runNpm(["pack", "--json", "--pack-destination", packDir]);
  const entry = parsePackEntry(output, "npm pack output");
  const tarball = path.join(packDir, entry.filename);
  assert.equal(fs.existsSync(tarball), true, `missing packed tarball: ${tarball}`);
  verifyTarballMetadata(entry, tarball);
  return { entry, tarball };
}

function parsePackEntry(json, source) {
  const entries = JSON.parse(json);
  assert.equal(Array.isArray(entries), true, `${source} must be a JSON array`);
  assert.equal(entries.length, 1, `${source} must describe exactly one tarball`);
  const [entry] = entries;
  assert.equal(typeof entry?.filename, "string", `${source} lacks filename`);
  assert.equal(Array.isArray(entry.files), true, `${source} lacks files`);
  return entry;
}

function verifyTarballMetadata(entry, tarball) {
  assert.equal(path.basename(tarball), entry.filename, "tarball filename differs from pack JSON");
  const bytes = fs.readFileSync(tarball);
  if (typeof entry.size === "number") {
    assert.equal(bytes.length, entry.size, "tarball size differs from pack JSON");
  }
  if (typeof entry.integrity === "string") {
    const integrity = `sha512-${crypto.createHash("sha512").update(bytes).digest("base64")}`;
    assert.equal(integrity, entry.integrity, "tarball integrity differs from pack JSON");
  }
  if (typeof entry.shasum === "string") {
    const shasum = crypto.createHash("sha1").update(bytes).digest("hex");
    assert.equal(shasum, entry.shasum, "tarball shasum differs from pack JSON");
  }
}

function parseOptions(args) {
  const options = {};
  for (let index = 0; index < args.length; index += 2) {
    const flag = args[index];
    const value = args[index + 1];
    assert.ok(value, `missing value for ${flag ?? "argument"}`);
    if (flag === "--tarball") options.tarball = value;
    else if (flag === "--pack-json") options.packJson = value;
    else throw new Error(`unknown argument: ${flag}`);
  }
  const supplied = Boolean(options.tarball || options.packJson);
  assert.equal(
    supplied && Boolean(options.tarball) === Boolean(options.packJson),
    supplied,
    "--tarball and --pack-json must be supplied together",
  );
  return options;
}

function suppliedArtifact(options) {
  const tarball = path.resolve(options.tarball);
  const packJson = path.resolve(options.packJson);
  assert.equal(fs.existsSync(tarball), true, `missing supplied tarball: ${tarball}`);
  assert.equal(fs.existsSync(packJson), true, `missing supplied pack JSON: ${packJson}`);
  const entry = parsePackEntry(fs.readFileSync(packJson, "utf8"), "supplied pack JSON");
  verifyTarballMetadata(entry, tarball);
  return { entry, tarball };
}

function verifyManifest(entry) {
  const files = entry.files.map((file) => file.path.replaceAll("\\", "/"));
  const protos = files.filter((file) => file.endsWith(".proto")).sort();
  const forbidden = files.filter(
    (file) =>
      file.startsWith("dist/src/") ||
      file.startsWith("dist/test/") ||
      file.startsWith(".build/") ||
      /(^|\/)sample-worker(?:\.|\/)/.test(file),
  );
  const failures = [];
  if (!files.includes("LICENSE")) {
    failures.push("missing LICENSE");
  }
  if (JSON.stringify(protos) !== JSON.stringify([...EXPECTED_PROTOS].sort())) {
    failures.push(`expected exactly seven bundled protos; found: ${protos.join(", ") || "none"}`);
  }
  if (forbidden.length > 0) {
    failures.push(`forbidden build/test/sample files: ${forbidden.join(", ")}`);
  }
  assert.deepEqual(failures, [], `invalid npm artifact:\n- ${failures.join("\n- ")}`);
}

function verifyPublint(tarball) {
  const cli = path.join(SDK_ROOT, "node_modules", "publint", "src", "cli.js");
  assert.equal(fs.existsSync(cli), true, `cannot locate publint CLI: ${cli}`);
  const output = run(process.execPath, [cli, tarball, "--strict"]);
  if (output) console.log(output);
}

function writeConsumer(consumerDir) {
  const files = {
    "package.json": JSON.stringify({ name: "cap-package-consumer", private: true, type: "module" }),
    "cjs-probe.cjs": `
const assert = require("node:assert/strict");
const sdk = require("cap-sdk-node");
(async () => {
  assert.equal(typeof sdk.loadRoot, "function");
  const packet = (await sdk.loadRoot()).lookupType("cordum.agent.v1.BusPacket");
  assert.equal(packet.fullName, ".cordum.agent.v1.BusPacket");
})().catch((error) => { console.error(error); process.exitCode = 1; });
`,
    "esm-probe.mjs": `
import assert from "node:assert/strict";
import { loadRoot } from "cap-sdk-node";
assert.equal(typeof loadRoot, "function");
const packet = (await loadRoot()).lookupType("cordum.agent.v1.BusPacket");
assert.equal(packet.fullName, ".cordum.agent.v1.BusPacket");
`,
    "consumer.ts": `
import { loadRoot, SUBJECT_SUBMIT } from "cap-sdk-node";
async function describePacket(): Promise<string> {
  const packet = (await loadRoot()).lookupType("cordum.agent.v1.BusPacket");
  return \`${"${SUBJECT_SUBMIT}"}:${"${packet.fullName}"}\`;
}
void describePacket();
`,
    "tsconfig.json": JSON.stringify({
      compilerOptions: {
        lib: ["DOM", "ES2020"],
        module: "NodeNext",
        moduleResolution: "NodeNext",
        noEmit: true,
        skipLibCheck: false,
        strict: true,
        target: "ES2020",
        types: ["node"],
      },
      include: ["*.ts"],
    }),
  };
  fs.mkdirSync(consumerDir, { recursive: true });
  for (const [name, content] of Object.entries(files)) {
    fs.writeFileSync(path.join(consumerDir, name), `${content.trim()}\n`);
  }
  for (const name of ["package-trust-smoke.cjs", "package-trust-consumer.ts"]) {
    fs.copyFileSync(path.join(SDK_ROOT, "scripts", name), path.join(consumerDir, name));
  }
}

function verifyConsumer(tarball, tempDir) {
  const consumerDir = path.join(tempDir, "consumer");
  writeConsumer(consumerDir);
  runNpm(
    ["install", "--ignore-scripts", "--no-audit", "--no-fund", "--package-lock=false", tarball],
    consumerDir,
  );
  const typescriptVersion = require(path.join(
    SDK_ROOT,
    "node_modules",
    "typescript",
    "package.json",
  )).version;
  const nodeTypesVersion = require(path.join(
    SDK_ROOT,
    "node_modules",
    "@types",
    "node",
    "package.json",
  )).version;
  runNpm(
    [
      "install",
      "--save-dev",
      "--ignore-scripts",
      "--no-audit",
      "--no-fund",
      "--package-lock=false",
      `typescript@${typescriptVersion}`,
      `@types/node@${nodeTypesVersion}`,
    ],
    consumerDir,
  );
  run(process.execPath, ["cjs-probe.cjs"], consumerDir);
  const trustOutput = run(process.execPath, ["package-trust-smoke.cjs"], consumerDir);
  if (trustOutput) console.log(trustOutput);
  run(process.execPath, ["esm-probe.mjs"], consumerDir);

  const tsc = path.join(consumerDir, "node_modules", "typescript", "bin", "tsc");
  run(process.execPath, [tsc, "--project", "tsconfig.json"], consumerDir);
}

function main() {
  const options = parseOptions(process.argv.slice(2));
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "cap-node-package-"));
  try {
    const artifact = options.tarball ? suppliedArtifact(options) : packSdk(tempDir);
    const { entry, tarball } = artifact;
    verifyManifest(entry);
    verifyPublint(tarball);
    verifyConsumer(tarball, tempDir);
    console.log(`package verification passed: ${entry.filename}`);
  } finally {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
}

try {
  main();
} catch (error) {
  console.error(error instanceof Error ? error.stack : error);
  process.exitCode = 1;
}
