import { expect } from "chai";
import {
  MAX_LEGACY_REDIS_KEY_BYTES,
  MAX_RESOURCE_AUTHORITY_BYTES,
  MAX_RESOURCE_IDENTIFIER_BYTES,
  MAX_RESOURCE_SIZE_BYTES,
  MAX_RESOURCE_URI_BYTES,
  ResourceInteger,
  ResourceRefInput,
  ResourceRefValidationError,
  canonicalLegacyRedisKey,
  validateResourceRef,
  validateResourceRefCompatibility,
} from "../src/production-resource";

const NOW = new Date("2026-07-20T12:00:00.000Z");

function validRef(): ResourceRefInput {
  return {
    resolverId: "redis",
    uri: "redis://ctx:job-1",
    sha256: Buffer.alloc(32, 1),
    mediaType: "application/json",
    sizeBytes: 128,
    expiresAt: { seconds: NOW.getTime() / 1000 + 3600, nanos: 0 },
    purpose: "job-input",
  };
}

describe("production resource validation", () => {
  it("accepts a bounded canonical reference", () => {
    expect(() => validateResourceRef(validRef(), ["redis"], NOW)).not.to.throw();
  });

  const invalidFields: ReadonlyArray<{
    name: string;
    mutate: (ref: ResourceRefInput) => void;
  }> = [
    { name: "empty resolver", mutate: (r) => { r.resolverId = ""; } },
    { name: "trimmed resolver", mutate: (r) => { r.resolverId = " redis"; } },
    { name: "invalid resolver", mutate: (r) => { r.resolverId = "redis/primary"; } },
    {
      name: "long resolver",
      mutate: (r) => { r.resolverId = "r".repeat(MAX_RESOURCE_IDENTIFIER_BYTES + 1); },
    },
    { name: "short digest", mutate: (r) => { r.sha256 = Buffer.alloc(31); } },
    { name: "long digest", mutate: (r) => { r.sha256 = Buffer.alloc(33); } },
    { name: "zero size", mutate: (r) => { r.sizeBytes = 0; } },
    { name: "oversize", mutate: (r) => { r.sizeBytes = MAX_RESOURCE_SIZE_BYTES + 1; } },
    { name: "empty media", mutate: (r) => { r.mediaType = ""; } },
    { name: "trimmed media", mutate: (r) => { r.mediaType = " application/json"; } },
    { name: "uppercase media", mutate: (r) => { r.mediaType = "Application/JSON"; } },
    {
      name: "media parameter",
      mutate: (r) => { r.mediaType = "application/json; charset=utf-8"; },
    },
    { name: "empty purpose", mutate: (r) => { r.purpose = ""; } },
    { name: "trimmed purpose", mutate: (r) => { r.purpose = " input"; } },
    { name: "invalid purpose", mutate: (r) => { r.purpose = "job input"; } },
    { name: "missing expiry", mutate: (r) => { r.expiresAt = undefined; } },
    { name: "invalid expiry", mutate: (r) => { r.expiresAt = { seconds: 1, nanos: -1 }; } },
    {
      name: "expired",
      mutate: (r) => { r.expiresAt = { seconds: NOW.getTime() / 1000, nanos: 0 }; },
    },
  ];

  for (const testCase of invalidFields) {
    it(`rejects ${testCase.name}`, () => {
      const ref = validRef();
      testCase.mutate(ref);
      expect(() => validateResourceRef(ref, ["redis"], NOW)).to.throw();
    });
  }

  for (const installed of [[], ["Redis"], ["redis "], ["other"], ["redis", " invalid"]]) {
    it(`rejects the non-exact resolver list ${JSON.stringify(installed)}`, () => {
      expect(() => validateResourceRef(validRef(), installed, NOW)).to.throw(/resolver/);
    });
  }

  it("rejects a string masquerading as the resolver list", () => {
    const installed = "redis" as unknown as readonly string[];
    expect(() => validateResourceRef(validRef(), installed, NOW)).to.throw(/resolver configuration/);
  });

  const unsafeUris = [
    "",
    " redis://ctx:job-1",
    "redis://ctx:job-1 ",
    "redis:ctx:job-1",
    "Redis://ctx:job-1",
    "1redis://ctx:job-1",
    "redis://",
    "redis:///job",
    `redis://${"a".repeat(MAX_RESOURCE_AUTHORITY_BYTES + 1)}`,
    "redis://user:secret@ctx/job",
    "redis://ctx/job?token=secret",
    "redis://ctx/job#secret",
    "redis://ctx/../secret",
    "redis://ctx/%2e%2e/secret",
    "redis://ctx/%252e%252e/secret",
    "redis://ctx/a\\b",
    "redis://ctx/%00",
    "redis://ctx/",
    "redis://ctx//job",
    "redis://ctx/./job",
    "redis://ctx/%2Fjob",
    `redis://ctx/${"a".repeat(MAX_RESOURCE_URI_BYTES)}`,
  ];

  for (const uri of unsafeUris) {
    it(`rejects the unsafe URI ${JSON.stringify(uri)}`, () => {
      const ref = validRef();
      ref.uri = uri;
      expect(() => validateResourceRef(ref, ["redis"], NOW)).to.throw(/URI/);
    });
  }

  it("accepts protobuf Long-like integer inputs", () => {
    const ref = validRef();
    ref.sizeBytes = { toString: () => "128" };
    ref.expiresAt = {
      seconds: { toString: () => String(NOW.getTime() / 1000 + 3600) },
      nanos: { toString: () => "0" },
    };
    expect(() => validateResourceRef(ref, ["redis"], NOW)).not.to.throw();
  });

  for (const size of [1.5, Number.MAX_SAFE_INTEGER + 1, "0128", "not-an-integer"]) {
    it(`rejects non-canonical integer ${JSON.stringify(size)}`, () => {
      const ref = validRef();
      ref.sizeBytes = size;
      expect(() => validateResourceRef(ref, ["redis"], NOW)).to.throw(/size/);
    });
  }

  it("normalizes a throwing Long-like value to a validation error", () => {
    const ref = validRef();
    ref.sizeBytes = { toString: () => { throw new Error("boom"); } };
    expect(() => validateResourceRef(ref, ["redis"], NOW)).to.throw(ResourceRefValidationError, /size/);
  });

  it("rejects a Long-like value returning a non-string", () => {
    const ref = validRef();
    ref.sizeBytes = { toString: () => 42 } as unknown as ResourceInteger;
    expect(() => validateResourceRef(ref, ["redis"], NOW)).to.throw(ResourceRefValidationError, /size/);
  });
});

describe("legacy Redis resource compatibility", () => {
  it("returns exact canonical key bytes", () => {
    expect(canonicalLegacyRedisKey("redis://res:job-1[2]")).to.deep.equal(
      Buffer.from("res:job-1[2]"),
    );
  });

  const invalidPointers = [
    "",
    "Redis://res:job",
    "redis://",
    " redis://res:job",
    "redis://res:job ",
    "redis://res/job",
    "redis://res\\job",
    "redis://res..job",
    "redis://res%3Ajob",
    "redis://user@res:job",
    "redis://res:job?token=x",
    "redis://res:job#part",
    "redis://res:\x00job",
    `redis://${"k".repeat(MAX_LEGACY_REDIS_KEY_BYTES + 1)}`,
  ];

  for (const pointer of invalidPointers) {
    it(`rejects the ambiguous pointer ${JSON.stringify(pointer)}`, () => {
      expect(() => canonicalLegacyRedisKey(pointer)).to.throw(/legacy Redis/);
    });
  }

  it("accepts matching dual fields and single-field migration", () => {
    expect(() => validateResourceRefCompatibility("redis://ctx:job-1", validRef())).not.to.throw();
    expect(() => validateResourceRefCompatibility("", validRef())).not.to.throw();
    expect(() => validateResourceRefCompatibility("redis://ctx:job-1", undefined)).not.to.throw();
  });

  const conflicts: ReadonlyArray<{
    legacy: string;
    resolver: string;
    uri: string;
  }> = [
    { legacy: "redis://ctx:other", resolver: "redis", uri: "redis://ctx:job-1" },
    { legacy: "redis://ctx:job-1", resolver: "blob", uri: "redis://ctx:job-1" },
    { legacy: "redis://ctx/../job-1", resolver: "redis", uri: "redis://ctx:job-1" },
    { legacy: "redis://ctx:job-1", resolver: "redis", uri: "redis://ctx:%6aob-1" },
  ];

  for (const conflict of conflicts) {
    it(`rejects conflicting dual fields ${JSON.stringify(conflict)}`, () => {
      const ref = validRef();
      ref.resolverId = conflict.resolver;
      ref.uri = conflict.uri;
      expect(() => validateResourceRefCompatibility(conflict.legacy, ref)).to.throw(/legacy/);
    });
  }
});
