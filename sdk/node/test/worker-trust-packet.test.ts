import * as assert from "node:assert";
import * as fs from "node:fs";
import * as path from "node:path";
import { Reader, Writer } from "protobufjs/minimal";

import { loadRoot } from "../src/protos";
import {
  AuthenticateTrustPacket,
  ChallengeRequestTrustPacket,
  ResultTrustPacket,
  WorkerTrustPacket,
  marshalWorkerTrustPacket,
  unmarshalWorkerTrustPacket,
  validateWorkerTrustPacket,
} from "../src/worker-trust";

type Phase = "challenge_request" | "challenge" | "authenticate" | "result";

function fixtureDir(): string {
  return path.resolve(__dirname, "..", "..", "..", "..", "spec", "conformance", "fixtures", "handshake");
}

function manifest(): Record<Phase, { packet: string }> {
  const value = JSON.parse(fs.readFileSync(path.join(fixtureDir(), "manifest.json"), "utf8"));
  return value.positive as Record<Phase, { packet: string }>;
}

async function fixture(phase: Phase): Promise<{ raw: Uint8Array; packet: WorkerTrustPacket }> {
  const raw = fs.readFileSync(path.join(fixtureDir(), manifest()[phase].packet));
  const type = (await loadRoot()).lookupType("cordum.agent.v1.BusPacket");
  return { raw, packet: type.decode(raw) as unknown as WorkerTrustPacket };
}

function nestedUnknown(raw: Uint8Array): Uint8Array {
  const source = Buffer.from(raw);
  const tag = Buffer.from([0x9a, 0x01]);
  const tagAt = source.indexOf(tag);
  assert.ok(tagAt >= 0, "challenge request payload tag missing");
  const lengthAt = tagAt + tag.length;
  const lengthReader = Reader.create(source.subarray(lengthAt));
  const oldLength = lengthReader.uint32();
  const payloadAt = lengthAt + lengthReader.pos;
  const payloadEnd = payloadAt + oldLength;
  const unknown = Writer.create().uint32(1000 << 3).uint32(1).finish();
  const newLength = Writer.create().uint32(oldLength + unknown.length).finish();
  return Buffer.concat([
    source.subarray(0, lengthAt),
    newLength,
    source.subarray(payloadAt, payloadEnd),
    unknown,
    source.subarray(payloadEnd),
  ]);
}

describe("worker trust packet contract", () => {
  it("accepts every signed positive phase and exact protocol v1", async () => {
    for (const phase of ["challenge_request", "challenge", "authenticate", "result"] as const) {
      const { packet } = await fixture(phase);
      assert.doesNotThrow(() => validateWorkerTrustPacket(packet));
      packet.protocolVersion = 2;
      assert.throws(() => validateWorkerTrustPacket(packet), /protocol version/i);
    }
  });

  it("rejects envelope and recursively nested object unknown fields", async () => {
    const { packet } = await fixture("challenge_request");
    (packet as unknown as Record<string, unknown>).attacker = "value";
    assert.throws(() => validateWorkerTrustPacket(packet), /unknown field/i);

    const { packet: nested } = await fixture("authenticate");
    (((nested as AuthenticateTrustPacket).workerHandshakeAuthenticate.challenge) as unknown as Record<string, unknown>).attacker = 1;
    assert.throws(() => validateWorkerTrustPacket(nested), /unknown field/i);
  });
});

describe("worker trust packet codec", () => {
  it("round-trips valid packets and rejects empty, oversized, and unknown wire fields", async () => {
    const { raw, packet } = await fixture("challenge_request");
    const encoded = await marshalWorkerTrustPacket(packet);
    assert.deepStrictEqual(Buffer.from(encoded), Buffer.from(raw));
    const decoded = await unmarshalWorkerTrustPacket(encoded);
    assert.strictEqual(
      (decoded as ChallengeRequestTrustPacket).workerHandshakeChallengeRequest.workerId,
      "worker-fixture"
    );

    await assert.rejects(() => unmarshalWorkerTrustPacket(new Uint8Array()), /size/i);
    await assert.rejects(
      () => unmarshalWorkerTrustPacket(new Uint8Array(65_537)),
      /size/i
    );
    const topUnknown = Buffer.concat([
      Buffer.from(raw),
      Buffer.from(Writer.create().uint32(1000 << 3).uint32(1).finish()),
    ]);
    await assert.rejects(() => unmarshalWorkerTrustPacket(topUnknown), /unknown field/i);
    await assert.rejects(() => unmarshalWorkerTrustPacket(nestedUnknown(raw)), /unknown field/i);
    await assert.rejects(
      () => unmarshalWorkerTrustPacket(Buffer.from(raw).subarray(0, raw.length - 1)),
      /length|malformed|decode/i
    );
    assert.strictEqual(raw[0], 0x0a);
    const nonMinimalTag = Buffer.concat([Buffer.from([0x8a, 0x00]), Buffer.from(raw).subarray(1)]);
    await assert.rejects(() => unmarshalWorkerTrustPacket(nonMinimalTag), /canonical|malformed/i);
  });

  it("rejects missing identities, nonce bounds, phase ambiguity, and invalid result shape", async () => {
    const { packet } = await fixture("challenge_request");
    const request = packet as ChallengeRequestTrustPacket;
    request.workerHandshakeChallengeRequest.clientNonce = new Uint8Array(31);
    assert.throws(() => validateWorkerTrustPacket(packet), /nonce/i);
    request.workerHandshakeChallengeRequest.clientNonce = new Uint8Array(32);
    request.senderId = "other-worker";
    assert.throws(() => validateWorkerTrustPacket(request), /sender/i);

    const { packet: ambiguous } = await fixture("challenge");
    (ambiguous as unknown as Record<string, unknown>).workerHandshakeResult = {};
    assert.throws(() => validateWorkerTrustPacket(ambiguous), /exactly one/i);

    const { packet: result } = await fixture("result");
    (result as ResultTrustPacket).authToken = "";
    assert.throws(() => validateWorkerTrustPacket(result), /token/i);
  });
});
