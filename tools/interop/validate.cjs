'use strict';
// Interop contract validator for CAP.
//
// Checks asyncapi/cap.asyncapi.yaml against the spec/09 subject table, the
// spec/14 worker-trust request/reply table, the messages in
// proto/cordum/agent/v1/*.proto and the subject constants in sdk/go/*.go.
//
// Usage, from the repository root:
//   npm ci --prefix tools/interop && node tools/interop/validate.cjs
//
// It prints one summary line per area and exits 0 only when every check
// passes. Every failure is printed as `FAIL <area>: <message>` and makes it
// exit 1. It never fetches anything: a non-local $ref fails before parsing.

const fs = require('node:fs');
const path = require('node:path');

const ROOT = path.resolve(__dirname, '..', '..');
const DOC = 'asyncapi/cap.asyncapi.yaml';
const PROTO_DIR = 'proto/cordum/agent/v1';
const GO_DIR = 'sdk/go';
const LEGACY_GO = 'sdk/go/legacy_handshake.go';
const PROTOBUF = 'application/x-protobuf';

const failures = [];
const summary = [];
const ctx = { raw: null, hv: null };

const fail = (area, msg) => failures.push(`FAIL ${area}: ${msg}`);
const errText = e => (e && e.message) || String(e);
const isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v);
const read = rel => fs.readFileSync(path.join(ROOT, rel), 'utf8').replace(/\r\n/g, '\n');
const readJSON = rel => JSON.parse(read(rel));

async function group(area, fn) {
  try {
    await fn();
  } catch (e) {
    fail(area, errText(e));
  }
}

// once caches a loader's value or its error, so every area sees the same input.
function once(fn) {
  let done = false;
  let value;
  let error;
  return () => {
    if (!done) {
      done = true;
      try { value = fn(); } catch (e) { error = e; }
    }
    if (error) throw error;
    return value;
  };
}

function loadModule(name) {
  try {
    return require(name);
  } catch (e) {
    fail('setup', `cannot load ${name} (${e.code || errText(e)}); run npm ci --prefix tools/interop`);
    return null;
  }
}

// mdTable returns the rows of the first pipe table after the exact heading
// line, keyed by header cell, with cells trimmed and backticks stripped.
function mdTable(file, heading, columns) {
  const lines = read(file).split('\n');
  let i = lines.indexOf(heading);
  if (i < 0) throw new Error(`${file}: heading "${heading}" not found`);
  for (i++; i < lines.length && !lines[i].startsWith('|'); i++) {
    if (lines[i].startsWith('#')) throw new Error(`${file}: no table under "${heading}"`);
  }
  const cells = [];
  for (; i < lines.length && lines[i].startsWith('|'); i++) {
    cells.push(lines[i].trim().replace(/^\||\|$/g, '').split('|').map(c => c.trim().replace(/`/g, '')));
  }
  const [header = [], sep = [], ...body] = cells;
  if (!sep.every(c => /^:?-+:?$/.test(c)) || body.length === 0) {
    throw new Error(`${file}: the table under "${heading}" has no rows`);
  }
  const missing = columns.filter(c => !header.includes(c));
  if (missing.length) throw new Error(`${file}: the table under "${heading}" lacks column(s) ${missing.join(', ')}`);
  return body.map(r => Object.fromEntries(header.map((h, j) => [h, r[j] === undefined ? '' : r[j]])));
}

const spec09 = once(() => mdTable('spec/09-transport-profile.md', '## Subject/Topic Conventions',
  ['Subject', 'Payload', 'Direction', 'Delivery'])
  .map(r => ({ ...r, address: r.Subject.replace(/<([^>]+)>/g, '{$1}') })));
const spec14 = once(() => mdTable('spec/14-capability-negotiation.md', '### Subjects and phases',
  ['Subject', 'Request payload', 'Reply payload']));

// proto returns the single proto package and every `<package>.<Message>` declared at column 0.
const proto = once(() => {
  const files = fs.readdirSync(path.join(ROOT, PROTO_DIR)).filter(f => f.endsWith('.proto')).sort();
  if (!files.length) throw new Error(`no .proto files in ${PROTO_DIR}`);
  const names = new Set();
  const pkgs = new Set();
  for (const f of files) {
    const text = read(`${PROTO_DIR}/${f}`);
    const pkg = /^package\s+([\w.]+)\s*;/m.exec(text);
    if (!pkg) throw new Error(`${PROTO_DIR}/${f} has no package line`);
    pkgs.add(pkg[1]);
    for (const m of text.matchAll(/^message\s+(\w+)\s*\{/gm)) names.add(`${pkg[1]}.${m[1]}`);
  }
  if (pkgs.size !== 1) throw new Error(`want one package in ${PROTO_DIR}, found ${[...pkgs].join(', ')}`);
  return { pkg: [...pkgs][0], names };
});

// goSubjects returns the "sys." string literals in the non-test top-level sdk/go files.
const goSubjects = once(() => {
  const out = new Set();
  const files = fs.readdirSync(path.join(ROOT, GO_DIR)).filter(f => f.endsWith('.go') && !f.endsWith('_test.go'));
  for (const f of files) {
    for (const line of read(`${GO_DIR}/${f}`).split('\n')) {
      if (line.trimStart().startsWith('//')) continue;
      for (const m of line.matchAll(/"(sys\.[^"\\]*)"/g)) out.add(m[1]);
    }
  }
  if (!out.size) throw new Error(`no "sys." string literals in ${GO_DIR}/*.go`);
  return out;
});

// legacySubjects returns the "sys." values of the top-level consts in legacy_handshake.go.
const legacySubjects = once(() => {
  const out = new Set();
  let block = false;
  for (const line of read(LEGACY_GO).split('\n')) {
    if (line === 'const (') { block = true; continue; }
    if (block && line === ')') { block = false; continue; }
    const re = block ? /^\t\w+(?:\s+\w+)?\s*=\s*"(sys\.[^"]*)"/ : /^const\s+\w+(?:\s+\w+)?\s*=\s*"(sys\.[^"]*)"/;
    const m = re.exec(line);
    if (m) out.add(m[1]);
  }
  if (!out.size) throw new Error(`no top-level "sys." const in ${LEGACY_GO}`);
  return out;
});

// deref follows local `#/` JSON-pointer $refs in the raw document.
function deref(node, depth = 0) {
  if (!isObj(node) || typeof node.$ref !== 'string') return node;
  const ref = node.$ref;
  if (depth > 16) throw new Error(`$ref ${ref} nests too deeply`);
  if (!ref.startsWith('#/')) throw new Error(`$ref ${ref} is not a local #/ reference`);
  let cur = ctx.raw;
  for (const token of ref.slice(2).split('/')) {
    const key = token.replace(/~1/g, '/').replace(/~0/g, '~');
    if (!isObj(cur) && !Array.isArray(cur)) throw new Error(`$ref ${ref} does not resolve`);
    if (!Object.prototype.hasOwnProperty.call(cur, key)) throw new Error(`$ref ${ref} does not resolve`);
    cur = cur[key];
  }
  return deref(cur, depth + 1);
}

function walk(node, at, visit) {
  if (Array.isArray(node)) node.forEach((v, i) => walk(v, [...at, i], visit));
  if (!isObj(node)) return;
  visit(node, at);
  for (const [k, v] of Object.entries(node)) walk(v, [...at, k], visit);
}

function channelList() {
  if (!isObj(ctx.raw.channels)) throw new Error('the document has no channels object');
  return Object.entries(ctx.raw.channels).map(([key, ch]) => ({ key, ch, address: isObj(ch) ? ch.address : undefined }));
}

function channelMessage(c) {
  const msgs = isObj(c.ch.messages) ? Object.values(c.ch.messages) : [];
  if (msgs.length !== 1) throw new Error(`channel ${c.address} has ${msgs.length} messages, want exactly 1`);
  const msg = deref(msgs[0]);
  if (!isObj(msg)) throw new Error(`channel ${c.address}: its message is not an object`);
  return msg;
}

async function checkAsyncapi(Parser) {
  const text = read(DOC);
  const refs = [...text.matchAll(/\$ref["']?\s*:\s*["']?([^"'\s,}\]]+)/g)].map(m => m[1]);
  const external = refs.filter(r => !r.startsWith('#'));
  if (external.length) throw new Error(`${DOC}: non-local $ref ${external.join(', ')} refused before parsing; nothing was fetched`);
  if (!Parser) throw new Error('@asyncapi/parser is not installed');
  const { diagnostics, extras } = await new Parser().parse(text, { source: DOC });
  const bad = diagnostics.filter(d => d.severity === 0 || d.severity === 1);
  for (const d of bad) fail('asyncapi', `${d.code} at ${(d.path || []).join('.')}: ${d.message}`);
  const raw = extras && extras.document && extras.document.data;
  if (!isObj(raw)) throw new Error(`${DOC}: the parser returned no raw document`);
  ctx.raw = raw;
  if (raw.asyncapi !== '3.1.0') fail('asyncapi', `${DOC} declares asyncapi ${raw.asyncapi}, want 3.1.0`);
  const pins = readJSON('tools/interop/package.json').dependencies;
  const installed = {};
  for (const name of ['@asyncapi/parser', 'ajv']) {
    installed[name] = readJSON(`tools/interop/node_modules/${name}/package.json`).version;
    if (installed[name] !== pins[name]) fail('asyncapi', `installed ${name} ${installed[name]} is not the pinned ${pins[name]}`);
  }
  summary.push(`asyncapi: ${DOC} is AsyncAPI ${raw.asyncapi} (@asyncapi/parser ${installed['@asyncapi/parser']}, ajv ${installed.ajv}); ${bad.length} error/warning diagnostics`);
}

function checkChannels() {
  const rows = spec09();
  const { pkg } = proto();
  const chans = channelList();
  const seen = new Map();
  for (const c of chans) {
    if (typeof c.address !== 'string') { fail('channels', `channel ${c.key} has no string address`); continue; }
    if (seen.has(c.address)) fail('channels', `channel address ${c.address} is used by both ${seen.get(c.address)} and ${c.key}`);
    seen.set(c.address, c.key);
  }
  const byAddr = new Map(rows.map(r => [r.address, r]));
  if (byAddr.size !== rows.length) fail('channels', 'the spec/09 subject table lists a subject twice');
  for (const r of rows) if (!seen.has(r.address)) fail('channels', `spec/09 subject ${r.address} has no channel`);
  for (const c of chans.filter(x => typeof x.address === 'string')) {
    const row = byAddr.get(c.address);
    if (!row) { fail('channels', `channel ${c.address} is not in the spec/09 subject table`); continue; }
    try {
      const msg = channelMessage(c);
      const want = `${pkg}.${row.Payload}`;
      if (msg.contentType !== PROTOBUF) fail('channels', `channel ${c.address}: message contentType ${msg.contentType}, want ${PROTOBUF}`);
      if (msg['x-cap-proto-message'] !== want) fail('channels', `channel ${c.address}: x-cap-proto-message ${msg['x-cap-proto-message']} does not equal ${want} (spec/09 Payload)`);
    } catch (e) {
      fail('channels', errText(e));
    }
  }
  summary.push(`channels: ${chans.length} (spec/09 subject table rows: ${rows.length}; request/reply subjects: ${spec14().length})`);
}

function checkProto() {
  const { names } = proto();
  let checked = 0;
  walk(ctx.raw, [], (node, at) => {
    for (const key of ['x-cap-proto-message', 'x-cap-reply-proto-message']) {
      if (!(key in node)) continue;
      checked++;
      if (!names.has(node[key])) fail('proto', `${at.join('.')}: ${key} ${node[key]} is not a message in ${PROTO_DIR}/*.proto`);
    }
  });
  if (checked === 0) fail('proto', 'the document has no x-cap-proto-message');
}

function checkOperation(key, op, rows, trust, pkg) {
  const ch = deref(isObj(op) ? op.channel : undefined);
  if (!isObj(ch)) throw new Error(`operation ${key}: its channel is not an object`);
  const row = rows.get(ch.address);
  if (op.action !== 'send') fail('operations', `operation ${key}: action ${op.action}, want send`);
  if (!row) return ch.address;
  for (const [ext, col] of [['x-cap-direction', 'Direction'], ['x-cap-delivery', 'Delivery']]) {
    if (op[ext] !== row[col]) fail('operations', `operation ${key}: ${ext} "${op[ext]}" does not equal the spec/09 ${col} "${row[col]}"`);
  }
  const t = trust.get(ch.address);
  const reply = op['x-cap-reply-proto-message'];
  if (!t) {
    if (reply !== undefined) fail('operations', `operation ${key}: x-cap-reply-proto-message on ${ch.address}, which is not a spec/14 request/reply subject`);
    return ch.address;
  }
  if (reply !== `${pkg}.${t['Reply payload']}`) fail('operations', `operation ${key}: x-cap-reply-proto-message ${reply} does not equal ${pkg}.${t['Reply payload']} (spec/14 Reply payload)`);
  if (t['Request payload'] !== row.Payload) fail('operations', `spec/14 request payload ${t['Request payload']} for ${ch.address} does not equal the spec/09 Payload ${row.Payload}`);
  return ch.address;
}

function checkOperations() {
  const rows = new Map(spec09().map(r => [r.address, r]));
  const trust = new Map(spec14().map(r => [r.Subject, r]));
  const { pkg } = proto();
  const ops = isObj(ctx.raw.operations) ? Object.entries(ctx.raw.operations) : [];
  const count = new Map();
  for (const [key, op] of ops) {
    try {
      const address = checkOperation(key, op, rows, trust, pkg);
      count.set(address, (count.get(address) || 0) + 1);
    } catch (e) {
      fail('operations', `operation ${key}: ${errText(e)}`);
    }
  }
  for (const c of channelList()) {
    const n = count.get(c.address) || 0;
    if (n !== 1) fail('operations', `channel ${c.address} has ${n} operations, want exactly 1`);
  }
  for (const s of trust.keys()) if (!rows.has(s)) fail('operations', `spec/14 subject ${s} is not in the spec/09 subject table`);
}

function checkSubjects() {
  const go = goSubjects();
  const legacy = legacySubjects();
  const addrs = new Set(channelList().map(c => c.address));
  const sys = [...addrs].filter(a => typeof a === 'string' && a.startsWith('sys.'));
  for (const a of sys) if (!go.has(a)) fail('subjects', `channel ${a} is not a subject string in ${GO_DIR}/*.go (unimplemented subject)`);
  const excluded = ctx.raw['x-cap-excluded-subjects'];
  if (!Array.isArray(excluded)) throw new Error('x-cap-excluded-subjects is not a list');
  const listed = new Set();
  for (const e of excluded) {
    if (!isObj(e) || typeof e.subject !== 'string' || typeof e.reason !== 'string' || !e.reason.trim()) {
      fail('subjects', `x-cap-excluded-subjects entry ${JSON.stringify(e)} needs a subject and a reason`);
      continue;
    }
    listed.add(e.subject);
    if (!legacy.has(e.subject)) fail('subjects', `x-cap-excluded-subjects lists ${e.subject}, which is not a deprecated const in ${LEGACY_GO}`);
    if (addrs.has(e.subject)) fail('subjects', `excluded subject ${e.subject} has a channel`);
  }
  for (const s of legacy) if (!listed.has(s)) fail('subjects', `legacy subject ${s} (${LEGACY_GO}) is missing from x-cap-excluded-subjects`);
  summary.push(`subjects: ${sys.length} sys.* channels, each a subject string in ${GO_DIR}; excluded legacy subjects: ${[...listed].sort().join(', ')}`);
}

function checkHeaders(Ajv) {
  const schemas = new Map();
  for (const c of channelList()) {
    try {
      const h = deref(channelMessage(c).headers);
      if (!isObj(h)) throw new Error(`channel ${c.address}: its message has no headers schema`);
      const k = JSON.stringify(h);
      schemas.set(k, (schemas.get(k) || 0) + 1);
    } catch (e) {
      fail('headers', errText(e));
    }
  }
  if (schemas.size !== 1) throw new Error(`channel messages use ${schemas.size} distinct headers schemas, want 1`);
  const schema = JSON.parse([...schemas.keys()][0]);
  if (schema.type !== 'object') fail('headers', `headers schema type ${schema.type}, want object`);
  for (const k of ['traceparent', 'tracestate']) {
    const p = isObj(schema.properties) ? schema.properties[k] : undefined;
    if (!isObj(p) || p.type !== 'string' || typeof p.pattern !== 'string') fail('headers', `headers property ${k} must be a string with a pattern`);
    if (Array.isArray(schema.required) && schema.required.includes(k)) fail('headers', `headers schema requires ${k}; trace headers are optional`);
  }
  if (!Ajv) throw new Error('ajv is not installed');
  ctx.hv = new Ajv({ allErrors: true }).compile(schema);
  summary.push(`headers: ${[...schemas.values()][0]} channel messages share one headers schema (optional traceparent, tracestate)`);
}

function report() {
  for (const s of summary) console.log(s);
  for (const f of failures) console.error(f);
  if (failures.length) {
    console.log(`interop validation: ${failures.length} failure(s)`);
    process.exitCode = 1;
  } else {
    console.log('interop validation: OK');
  }
}

async function main() {
  const parser = loadModule('@asyncapi/parser');
  const ajv = loadModule('ajv');
  const Ajv = ajv && (ajv.default || ajv);
  await group('asyncapi', () => checkAsyncapi(parser && parser.Parser));
  if (ctx.raw) {
    await group('channels', checkChannels);
    await group('proto', checkProto);
    await group('operations', checkOperations);
    await group('subjects', checkSubjects);
    await group('headers', () => checkHeaders(Ajv));
  }
  report();
}

main().catch(e => {
  fail('internal', errText(e));
  report();
});
