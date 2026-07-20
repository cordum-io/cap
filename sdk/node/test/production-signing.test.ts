/**
 * Node consumer for the cross-language CAP-PRODUCTION conformance vectors.
 *
 * Reads the SAME test/fixtures/production-signing-v1.json as
 * sdk/go/production_signing_vectors_test.go and
 * sdk/python/tests/test_production_signing.py. All three must reach identical
 * verdicts on every vector; that agreement is what makes the file a conformance
 * artifact rather than three unrelated local suites.
 */

import { expect } from "chai";
import crypto from "crypto";
import fs from "fs";
import path from "path";
import type { Type } from "protobufjs";
import { loadRoot } from "../src/protos";
import {
  InMemoryReplayStore,
  ReplayConflictError,
  ReplayOutcome,
} from "../src/production-replay";
import {
  extractProductionSignature,
  PRODUCTION_SIGNATURE_DOMAIN,
  productionPreimageDigest,
  verifyProductionPacket,
  verifyProductionSignature,
} from "../src/production-signing";
import type { ProductionTrustStore } from "../src/production-signing-types";
import { IdentityMismatchError, validateIdentityBinding } from "../src/production-validation";

interface SigningVector {
  name: string;
  expect: string;
  reject_reason?: string;
  trust_key_ids?: string[];
  raw_base64: string;
  unsigned_base64: string;
  signature_base64: string;
  preimage_digest_hex: string;
  body_digest_hex: string;
  message_id_hex: string;
  audience: string;
  key_id: string;
  expires_at_rfc3339: string;
}

interface Fixture {
  schema_version: number;
  domain_base64: string;
  verify_at_rfc3339: string;
  public_key_pem: string;
  raw_base64: string;
  unsigned_base64: string;
  signature_base64: string;
  preimage_digest_hex: string;
  trust: { audience: string; tenant: string; sender: string; publicKeys?: never;
           public_keys: Record<string, string> };
  vectors: SigningVector[];
  replay_vectors: { name: string; sequence: { vector: string; expect: string }[] }[];
  identity_binding_vectors: {
    name: string; expect: string;
    job_request_base64: string; authoritative_base64?: string;
  }[];
}

const fixture = JSON.parse(
  fs.readFileSync(
    path.resolve(__dirname, "../../../../test/fixtures/production-signing-v1.json"),
    "utf8",
  ),
) as Fixture;

// Fixture reason -> the message Node uses. Go collapses an identity mismatch
// into "unknown key id" so the error cannot serve as an oracle; Node names the
// specific check. The fixture records the OUTCOME and each SDK maps it.
const REASON_MESSAGES: Record<string, string[]> = {
  invalid_signature: ["invalid signature"],
  audience_mismatch: ["audience mismatch"],
  signature_expired: ["signature expired"],
  unknown_key_id: ["unknown key id"],
  identity_mismatch: ["tenant mismatch", "sender mismatch"],
};

function trustFor(vector: SigningVector): ProductionTrustStore {
  const installed = vector.trust_key_ids ?? Object.keys(fixture.trust.public_keys);
  const publicKeys: Record<string, string> = {};
  for (const keyId of installed) {
    const pem = fixture.trust.public_keys[keyId];
    if (!pem) throw new Error(`vector ${vector.name} names unknown trust key ${keyId}`);
    publicKeys[keyId] = pem;
  }
  return {
    audience: fixture.trust.audience,
    tenant: fixture.trust.tenant,
    sender: fixture.trust.sender,
    publicKeys,
    nowMs: () => Date.parse(fixture.verify_at_rfc3339),
  };
}

describe("CAP-PRODUCTION cross-language conformance vectors", () => {
  let BusPacket: Type;
  let JobRequest: Type;
  let IdentityBinding: Type;

  before(async () => {
    const root = await loadRoot();
    BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    JobRequest = root.lookupType("cordum.agent.v1.JobRequest");
    IdentityBinding = root.lookupType("cordum.agent.v1.IdentityBinding");
  });

  it("loads a fixture that has not lost coverage", () => {
    // A shrinking fixture silently weakens all three SDKs at once.
    expect(fixture.schema_version).to.be.at.least(2);
    expect(fixture.vectors.length).to.be.at.least(9);
    expect(fixture.replay_vectors.length).to.be.at.least(3);
    expect(fixture.identity_binding_vectors.length).to.be.at.least(3);
    expect(Buffer.from(fixture.domain_base64, "base64").equals(PRODUCTION_SIGNATURE_DOMAIN))
      .to.equal(true, "fixture domain separator disagrees with the SDK's");
  });

  describe("verdicts", () => {
    for (const vector of fixture.vectors) {
      it(`${vector.expect}s ${vector.name}`, () => {
        const raw = Buffer.from(vector.raw_base64, "base64");
        if (vector.expect === "accept") {
          const packet = verifyProductionPacket(raw, BusPacket, trustFor(vector)) as any;
          expect(packet.signatureMetadata.audience).to.equal(vector.audience);
          expect(packet.signatureMetadata.keyId).to.equal(vector.key_id);
          expect(Buffer.from(packet.signatureMetadata.messageId).toString("hex"))
            .to.equal(vector.message_id_hex);
          return;
        }
        let thrown: Error | undefined;
        try {
          verifyProductionPacket(raw, BusPacket, trustFor(vector));
        } catch (err) {
          thrown = err as Error;
        }
        expect(thrown, "labeled reject but the verifier accepted it").to.not.equal(undefined);
        const candidates = REASON_MESSAGES[vector.reject_reason ?? ""];
        expect(candidates, `unknown reject_reason ${vector.reject_reason}`).to.not.equal(undefined);
        expect(
          candidates.some((c) => thrown!.message.includes(c)),
          `reason ${vector.reject_reason} expected one of ${candidates}, got "${thrown!.message}"`,
        ).to.equal(true);
      });
    }
  });

  describe("digests", () => {
    for (const vector of fixture.vectors) {
      // The domain-separated SIGNATURE preimage and the undomained signed-BODY
      // digest are different values over identical bytes. Swapping them makes a
      // valid redelivery look like a replay conflict to the other SDKs.
      it(`pins both digests for ${vector.name}`, () => {
        const raw = Buffer.from(vector.raw_base64, "base64");
        const extracted = extractProductionSignature(raw);
        expect(Buffer.from(extracted.unsigned).toString("base64")).to.equal(vector.unsigned_base64);
        expect(Buffer.from(extracted.signature).toString("base64"))
          .to.equal(vector.signature_base64);
        expect(Buffer.from(productionPreimageDigest(extracted.unsigned)).toString("hex"))
          .to.equal(vector.preimage_digest_hex);
        expect(crypto.createHash("sha256").update(Buffer.from(extracted.unsigned))
          .digest("hex")).to.equal(vector.body_digest_hex);
        expect(vector.preimage_digest_hex).to.not.equal(vector.body_digest_hex);
      });
    }
  });

  it("verifies the baseline signature against the recorded public key", () => {
    const baseline = fixture.vectors.find((v) => v.name === "accept/baseline")!;
    const raw = Buffer.from(baseline.raw_base64, "base64");
    expect(Buffer.from(verifyProductionSignature(raw, fixture.public_key_pem)).toString("base64"))
      .to.equal(baseline.unsigned_base64);
  });

  it("rejects a duplicate signature field", () => {
    const raw = Buffer.from(fixture.raw_base64, "base64");
    const duplicate = Buffer.concat([raw, Buffer.from([0x72, 0x01, 0x00])]);
    expect(() => extractProductionSignature(duplicate)).to.throw("duplicate");
  });

  describe("replay sequences", () => {
    for (const replay of fixture.replay_vectors) {
      it(replay.name, async () => {
        const byName = new Map(fixture.vectors.map((v) => [v.name, v]));
        const store = new InMemoryReplayStore();
        for (const [index, step] of replay.sequence.entries()) {
          const vector = byName.get(step.vector)!;
          const admit = () => store.admit(
            fixture.trust.tenant,
            vector.audience,
            fixture.trust.sender,
            Buffer.from(vector.message_id_hex, "hex"),
            Buffer.from(vector.body_digest_hex, "hex"),
            Date.parse(vector.expires_at_rfc3339),
          );
          if (step.expect === "conflict") {
            expect(admit).to.throw(ReplayConflictError);
            continue;
          }
          const want = step.expect === "first" ? ReplayOutcome.First : ReplayOutcome.Duplicate;
          expect(await admit(), `step ${index} of ${replay.name}`).to.equal(want);
        }
      });
    }
  });

  describe("identity binding", () => {
    for (const vector of fixture.identity_binding_vectors) {
      it(`${vector.expect}s ${vector.name}`, () => {
        const request = JobRequest.decode(
          Buffer.from(vector.job_request_base64, "base64"),
        ) as unknown as Record<string, unknown>;
        const authoritative = vector.authoritative_base64
          ? (IdentityBinding.decode(
              Buffer.from(vector.authoritative_base64, "base64"),
            ) as unknown as Record<string, unknown>)
          : undefined;
        if (vector.expect === "accept") {
          validateIdentityBinding(request, authoritative);
          return;
        }
        expect(() => validateIdentityBinding(request, authoritative))
          .to.throw(IdentityMismatchError);
      });
    }
  });

  it("keeps the legacy flat keys aliased to accept/baseline", () => {
    const baseline = fixture.vectors.find((v) => v.name === "accept/baseline")!;
    expect(fixture.raw_base64).to.equal(baseline.raw_base64);
    expect(fixture.unsigned_base64).to.equal(baseline.unsigned_base64);
    expect(fixture.signature_base64).to.equal(baseline.signature_base64);
    expect(fixture.preimage_digest_hex).to.equal(baseline.preimage_digest_hex);
    expect(fixture.public_key_pem).to.equal(fixture.trust.public_keys[baseline.key_id]);
  });
});
