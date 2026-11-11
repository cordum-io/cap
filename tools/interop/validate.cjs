'use strict';
// Interop contract validator for CAP.
//
// Checks asyncapi/cap.asyncapi.yaml against the spec/09 subject table, the
// spec/14 worker-trust request/reply table, the messages in
// proto/cordum/agent/v1/*.proto and the subject constants in sdk/go/*.go,
// then checks the trace-context, CloudEvents and A2A vectors in
// spec/interop/vectors/ (shape, header patterns, recomputed event ids,
// redaction, A2A TaskStates).
//
// Usage, from the repository root:
//   npm ci --prefix tools/interop && node tools/interop/validate.cjs
//
// It prints one summary line per area and exits 0 only when every check
// passes. Every failure is printed as `FAIL <area>: <message>` and makes it
// exit 1. It never fetches anything: a non-local $ref fails before parsing.

const crypto = require('node:crypto');
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

// ---- Vectors ---------------------------------------------------------------

const TRACE_VECTORS = 'spec/interop/vectors/trace-context-v1.json';
const CE_VECTORS = 'spec/interop/vectors/cloudevents-v1.json';
const A2A_VECTORS = 'spec/interop/vectors/a2a-v1.json';

// The redaction list of spec/interop/cloudevents-mapping.md ("Redaction list"),
// proto and JSON spellings. The vectors must list at least these.
const REQUIRED_REDACTED = ['auth_token', 'authToken', 'signature', 'signature_metadata', 'signatureMetadata',
  'identity', 'env', 'labels', 'error_message', 'errorMessage'];

// A2A v1.0.0 specification/a2a.proto, enum TaskState (lines 186-208).
const A2A_STATES = ['TASK_STATE_UNSPECIFIED', 'TASK_STATE_SUBMITTED', 'TASK_STATE_WORKING', 'TASK_STATE_COMPLETED',
  'TASK_STATE_FAILED', 'TASK_STATE_CANCELED', 'TASK_STATE_INPUT_REQUIRED', 'TASK_STATE_REJECTED',
  'TASK_STATE_AUTH_REQUIRED'];

// RFC 8785 JCS. Byte-identical to canonicalize 2.1.0 for these vectors, whose
// numbers are all 32-bit integers.
const jcs = v => (v === null || typeof v !== 'object' ? JSON.stringify(v)
  : Array.isArray(v) ? `[${v.map(jcs).join(',')}]`
    : `{${Object.keys(v).sort().map(k => `${JSON.stringify(k)}:${jcs(v[k])}`).join(',')}}`);

const obj = (properties, required = Object.keys(properties)) => ({ type: 'object', required, properties });
const str = { type: 'string' };
const SPAN_MAP = obj({ input: { type: 'object' }, span_name: str, span_kind: { enum: ['PRODUCER', 'CONSUMER'] }, attributes: { type: 'object' } });
const TRACE_SCHEMA = obj({
  schema_version: { const: 1 },
  traceparent_cases: {
    type: 'array',
    minItems: 1,
    items: {
      ...obj({
        name: { type: 'string', minLength: 1 },
        headers: { type: 'object', additionalProperties: false, properties: { traceparent: str, tracestate: str } },
        verdict: { enum: ['valid', 'invalid'] },
        expected: {},
      }),
      if: { type: 'object', properties: { verdict: { const: 'valid' } } },
      then: {
        type: 'object',
        properties: {
          expected: obj({ trace_id: { type: 'string', pattern: '^[0-9a-f]{32}$' }, parent_id: { type: 'string', pattern: '^[0-9a-f]{16}$' }, sampled: { type: 'boolean' }, tracestate: str }),
        },
      },
      else: { type: 'object', properties: { expected: { type: 'null' } } },
    },
  },
  attribute_maps: obj({ publish: SPAN_MAP, process: SPAN_MAP }),
});
const CE_SCHEMA = obj({
  schema_version: { const: 1 },
  mapper_config: obj({ source: str }),
  redacted_keys: { type: 'array', minItems: 1, uniqueItems: true, items: str },
  cases: {
    type: 'array',
    minItems: 8,
    items: obj({
      name: { type: 'string', minLength: 1 },
      covers: { type: 'array', items: str },
      input: obj({ nats_subject: str, admission: { enum: ['production', 'legacy'] }, packet: { type: 'object' } }),
      expected_cloudevent: {
        ...obj({ id: str, source: str, specversion: { type: 'string', const: '1.0' }, type: str }),
        propertyNames: { type: 'string', pattern: '^[a-z0-9]{1,20}$' },
      },
    }),
  },
  unmapped_cases: { type: 'array', items: obj({ name: { type: 'string', minLength: 1 }, input: { type: 'object' }, expected: { const: 'not_mapped' } }) },
});

// checkShape validates data against an inline schema and names the case of each error.
function checkShape(area, Ajv, schema, data, lists) {
  if (!Ajv) throw new Error('ajv is not installed');
  const validate = new Ajv({ allErrors: true }).compile(schema);
  if (validate(data)) return true;
  for (const e of validate.errors) {
    const m = /^\/(\w+)\/(\d+)(\/.*)?$/.exec(e.instancePath);
    const item = m && lists.includes(m[1]) && Array.isArray(data[m[1]]) ? data[m[1]][Number(m[2])] : undefined;
    const label = isObj(item) && (item.name || item.id) ? `case ${item.name || item.id}${m[3] ? ` at ${m[3]}` : ''}` : (e.instancePath || 'document');
    const named = e.params && (e.params.additionalProperty || e.params.propertyName);
    const extra = named ? ` (${named})` : '';
    fail(area, `${label}: ${e.message}${extra}`);
  }
  return false;
}

function uniqueNames(area, items, key) {
  const seen = new Set();
  for (const it of items) {
    if (seen.has(it[key])) fail(area, `case ${it[key]}: duplicate ${key}`);
    seen.add(it[key]);
  }
}

function checkTraceVectors(Ajv) {
  const d = readJSON(TRACE_VECTORS);
  if (!checkShape('trace-context', Ajv, TRACE_SCHEMA, d, ['traceparent_cases'])) return;
  if (!ctx.hv) throw new Error('no TraceContextHeaders validator (see the headers area)');
  const cases = d.traceparent_cases;
  uniqueNames('trace-context', cases, 'name');
  let checks = 0;
  let agree = 0;
  const expect = (c, h, want, what) => {
    checks++;
    if (ctx.hv(h) === want) { agree++; return; }
    fail('trace-context', `case ${c.name}: TraceContextHeaders ${want ? 'rejects' : 'accepts'} ${what} ${JSON.stringify(h)}`);
  };
  for (const c of cases.filter(x => x.verdict === 'valid')) {
    const e = c.expected;
    const h = { traceparent: `00-${e.trace_id}-${e.parent_id}-${e.sampled ? '01' : '00'}` };
    if (e.tracestate !== '') h.tracestate = e.tracestate;
    expect(c, h, true, 'the producer headers');
  }
  for (const c of cases) {
    const tp = c.headers.traceparent;
    if (typeof tp === 'string' && tp.startsWith('00-')) {
      expect(c, { traceparent: tp }, c.verdict === 'valid', `${c.verdict === 'valid' ? 'a valid' : 'an invalid'} traceparent`);
    }
  }
  if (checks === 0) fail('trace-context', 'no header check ran');
  const valid = cases.filter(c => c.verdict === 'valid').length;
  summary.push(`trace-context vectors: ${cases.length} cases (${valid} valid, ${cases.length - valid} invalid); ${agree} header checks agree with TraceContextHeaders`);
}

function checkEventId(c) {
  const e = c.expected_cloudevent;
  if (c.input.admission === 'legacy') {
    const members = {};
    for (const k of ['type', 'subject', 'time', 'data']) if (k in e) members[k] = e[k];
    const want = crypto.createHash('sha256').update(jcs(members), 'utf8').digest('hex');
    if (e.id !== want) fail('cloudevents', `case ${c.name}: id ${e.id} does not equal the recomputed JCS id ${want}`);
    return;
  }
  const sm = isObj(c.input.packet.signatureMetadata) ? c.input.packet.signatureMetadata : {};
  const mid = sm.messageId;
  const buf = typeof mid === 'string' ? Buffer.from(mid, 'base64') : Buffer.alloc(0);
  if (buf.length !== 16 || buf.toString('base64') !== mid) {
    fail('cloudevents', `case ${c.name}: input.packet.signatureMetadata.messageId ${mid} is not the base64 of exactly 16 bytes`);
  } else if (e.id !== buf.toString('hex')) {
    fail('cloudevents', `case ${c.name}: id ${e.id} does not equal the messageId hex ${buf.toString('hex')}`);
  }
}

function checkCloudEventVectors(Ajv) {
  const d = readJSON(CE_VECTORS);
  const shapeOk = checkShape('cloudevents', Ajv, CE_SCHEMA, d, ['cases', 'unmapped_cases']);
  const redacted = new Set(isObj(d) && Array.isArray(d.redacted_keys) ? d.redacted_keys : []);
  for (const k of REQUIRED_REDACTED) if (!redacted.has(k)) fail('cloudevents', `redacted_keys is missing ${k}`);
  if (!shapeOk) return;
  uniqueNames('cloudevents', [...d.cases, ...d.unmapped_cases], 'name');
  for (const c of d.cases) {
    checkEventId(c);
    walk(c.expected_cloudevent, [], (node, at) => {
      for (const k of Object.keys(node)) if (redacted.has(k)) fail('cloudevents', `case ${c.name}: redacted key ${k} at ${[...at, k].join('.')}`);
    });
  }
  const legacy = d.cases.filter(c => c.input.admission === 'legacy').length;
  const production = d.cases.length - legacy;
  if (!legacy || !production) fail('cloudevents', `need at least one legacy and one production case, found ${legacy} and ${production}`);
  summary.push(`cloudevents vectors: ${d.cases.length} cases (${legacy} legacy JCS ids and ${production} production ids recomputed), ${d.unmapped_cases.length} unmapped cases`);
}

function checkA2AVectors() {
  const d = readJSON(A2A_VECTORS);
  if (d.a2a_version !== '1.0.0') fail('a2a', `a2a_version ${d.a2a_version}: TaskState list is pinned to A2A 1.0.0`);
  if (!Array.isArray(d.cases) || d.cases.length === 0) throw new Error(`${A2A_VECTORS} has no cases`);
  const cases = d.cases.filter(c => isObj(c) && typeof c.id === 'string' && c.id !== '');
  if (cases.length !== d.cases.length) fail('a2a', `${d.cases.length - cases.length} case(s) have no id`);
  uniqueNames('a2a', cases, 'id');
  const states = new Set(A2A_STATES);
  let bad = 0;
  for (const c of cases) {
    if (!states.has(c.expected_a2a_state)) { bad++; fail('a2a', `case ${c.id}: expected_a2a_state ${c.expected_a2a_state} is not an A2A 1.0 TaskState`); }
    if (c.expected_event_order === undefined) continue;
    if (!Array.isArray(c.expected_event_order)) { bad++; fail('a2a', `case ${c.id}: expected_event_order is not a list`); continue; }
    c.expected_event_order.forEach((s, i) => {
      if (states.has(s) && s !== 'TASK_STATE_UNSPECIFIED') return;
      bad++;
      fail('a2a', `case ${c.id}: expected_event_order[${i}] ${s} is not an emitted A2A 1.0 TaskState (the bridge never emits UNSPECIFIED)`);
    });
  }
  summary.push(`a2a vectors: ${d.cases.length} cases; ${bad ? `${bad} state(s) are not` : 'every state is one of'} the 9 A2A 1.0 TaskStates`);
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
  await group('trace-context', () => checkTraceVectors(Ajv));
  await group('cloudevents', () => checkCloudEventVectors(Ajv));
  await group('a2a', checkA2AVectors);
  report();
}

main().catch(e => {
  fail('internal', errText(e));
  report();
});
