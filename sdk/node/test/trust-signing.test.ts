import assert from "node:assert";
import * as crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import type { Type } from "protobufjs";

import { loadRoot } from "../src/protos";
import {
  encodeWorkerTrustUnsigned,
  signWorkerTrustPacket,
  verifyWorkerTrustPacket,
  workerTrustDigest,
  workerTrustDomain,
  workerTrustPhase,
  type WorkerTrustPacket,
  type WorkerTrustPhase,
} from "../src/trust-signing";

interface PositiveVector {
  digest_sha256: string;
  domain: string;
  packet: string;
  signer: "worker" | "scheduler";
}

interface Manifest {
  negative_vectors: unknown[];
  positive: Record<WorkerTrustPhase, PositiveVector>;
}

const PHASES: WorkerTrustPhase[] = [
  "challenge_request",
  "challenge",
  "authenticate",
  "result",
];
const FIELD_BY_PHASE: Record<WorkerTrustPhase, string> = {
  challenge_request: "workerHandshakeChallengeRequest",
  challenge: "workerHandshakeChallenge",
  authenticate: "workerHandshakeAuthenticate",
  result: "workerHandshakeResult",
};

function fixtureDir(): string {
  return path.resolve(
    __dirname,
    "..",
    "..",
    "..",
    "..",
    "spec",
    "conformance",
    "fixtures",
    "handshake"
  );
}

const manifest = JSON.parse(
  fs.readFileSync(path.join(fixtureDir(), "manifest.json"), "utf8")
) as Manifest;

async function busPacketType(): Promise<Type> {
  return (await loadRoot()).lookupType("cordum.agent.v1.BusPacket");
}

async function loadPacket(phase: WorkerTrustPhase): Promise<WorkerTrustPacket> {
  const type = await busPacketType();
  return type.decode(
    fs.readFileSync(path.join(fixtureDir(), manifest.positive[phase].packet))
  ) as unknown as WorkerTrustPacket;
}

function configuredKeys(): Record<string, string> {
  return {
    "worker-key-v1": fs.readFileSync(
      path.join(fixtureDir(), "worker_public.pem"),
      "utf8"
    ),
    "scheduler-key-v1": fs.readFileSync(
      path.join(fixtureDir(), "scheduler_public.pem"),
      "utf8"
    ),
  };
}

function asRecord(value: unknown): Record<string, unknown> {
  assert.ok(value && typeof value === "object");
  return value as Record<string, unknown>;
}

function payloadRecord(
  packet: WorkerTrustPacket,
  phase: WorkerTrustPhase
): Record<string, unknown> {
  return asRecord(asRecord(packet)[FIELD_BY_PHASE[phase]]);
}

function proofRecord(
  packet: WorkerTrustPacket,
  phase: WorkerTrustPhase
): Record<string, unknown> {
  const payload = payloadRecord(packet, phase);
  return phase === "authenticate" || phase === "result"
    ? asRecord(payload.challenge)
    : payload;
}

async function rejectsOrReturnsFalse(
  verify: () => Promise<boolean>
): Promise<boolean> {
  try {
    return await verify();
  } catch {
    return false;
  }
}

describe("worker trust signing vectors", () => {
  it("covers the four positive phases and the complete negative inventory", () => {
    assert.deepStrictEqual(Object.keys(manifest.positive).sort(), [...PHASES].sort());
    assert.strictEqual(manifest.negative_vectors.length, 38);
  });

  for (const phase of PHASES) {
    it(`reproduces the ${phase} digest and reference signature`, async () => {
      const packet = await loadPacket(phase);
      assert.strictEqual(workerTrustPhase(packet), phase);
      assert.strictEqual(workerTrustDomain(phase), manifest.positive[phase].domain);
      assert.strictEqual(
        Buffer.from(await workerTrustDigest(packet)).toString("hex"),
        manifest.positive[phase].digest_sha256
      );
      assert.strictEqual(await verifyWorkerTrustPacket(packet, configuredKeys()), true);
    });
  }

  it("clears only signature and emits auth_token before the result payload", async () => {
    const packet = await loadPacket("result");
    const unsigned = Buffer.from(await encodeWorkerTrustUnsigned(packet));
    const changedSignature = {
      ...packet,
      signature: new Uint8Array(64).fill(0x5a),
    } as WorkerTrustPacket;
    assert.deepStrictEqual(
      Buffer.from(await encodeWorkerTrustUnsigned(changedSignature)),
      unsigned
    );
    const authTag = unsigned.indexOf(Buffer.from([0x92, 0x01]));
    const payloadTag = unsigned.indexOf(Buffer.from([0xb2, 0x01]));
    assert.ok(authTag >= 0 && payloadTag > authTag);
  });

  it("rejects signed-field and auth_token tampering", async () => {
    const vectors: Array<{
      phase: WorkerTrustPhase;
      mutate: (packet: WorkerTrustPacket) => void;
    }> = [
      {
        phase: "challenge_request",
        mutate: (packet) => {
          payloadRecord(packet, "challenge_request").workerId = "worker-attacker";
        },
      },
      {
        phase: "challenge",
        mutate: (packet) => {
          payloadRecord(packet, "challenge").tenantId = "tenant-attacker";
        },
      },
      {
        phase: "authenticate",
        mutate: (packet) => {
          asRecord(payloadRecord(packet, "authenticate").capabilityHandshake)
            .agentName = "Tampered Worker";
        },
      },
      {
        phase: "result",
        mutate: (packet) => {
          asRecord(packet).authToken = "tampered-session-token";
        },
      },
    ];
    for (const vector of vectors) {
      const packet = await loadPacket(vector.phase);
      vector.mutate(packet);
      assert.strictEqual(
        await rejectsOrReturnsFalse(() => verifyWorkerTrustPacket(packet, configuredKeys())),
        false
      );
    }
  });

  it("uses configured key IDs and rejects unsafe key or signature inputs", async () => {
    const packet = await loadPacket("challenge_request");
    await assert.rejects(() => verifyWorkerTrustPacket(packet, {}), /key id/i);
    asRecord(packet).publicKey = configuredKeys()["worker-key-v1"];
    await assert.rejects(() => verifyWorkerTrustPacket(packet, {}), /unknown field/i);
    delete asRecord(packet).publicKey;

    const p384 = crypto.generateKeyPairSync("ec", { namedCurve: "secp384r1" });
    await assert.rejects(
      () => verifyWorkerTrustPacket(packet, { "worker-key-v1": p384.publicKey }),
      /P-256/
    );

    asRecord(packet).signature = new Uint8Array();
    await assert.rejects(() => verifyWorkerTrustPacket(packet, configuredKeys()), /signature/i);
    asRecord(packet).signature = new Uint8Array(64).fill(1);
    assert.strictEqual(await verifyWorkerTrustPacket(packet, configuredKeys()), false);
  });

  it("rejects unspecified or unknown proof algorithms in every phase", async () => {
    for (const phase of PHASES) {
      for (const algorithm of [0, 999]) {
        const packet = await loadPacket(phase);
        proofRecord(packet, phase).proofAlgorithm = algorithm;
        await assert.rejects(() => workerTrustDigest(packet), /proof algorithm/i);
        await assert.rejects(
          () => verifyWorkerTrustPacket(packet, configuredKeys()),
          /proof algorithm/i
        );
        await assert.rejects(
          () => signWorkerTrustPacket(packet, {}),
          /proof algorithm/i
        );
      }
    }
  });

  it("signs with the packet-selected P-256 key and strict DER", async () => {
    const packet = await loadPacket("challenge_request");
    const payload = payloadRecord(packet, "challenge_request");
    payload.proofKeyId = "ephemeral-key";
    asRecord(packet).signature = new Uint8Array();
    const p256 = crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
    const signed = await signWorkerTrustPacket(packet, {
      "ephemeral-key": p256.privateKey,
    });
    assert.strictEqual(Buffer.from(signed.signature ?? [])[0], 0x30);
    assert.strictEqual(
      await verifyWorkerTrustPacket(signed, { "ephemeral-key": p256.publicKey }),
      true
    );

    const p384 = crypto.generateKeyPairSync("ec", { namedCurve: "secp384r1" });
    await assert.rejects(
      () => signWorkerTrustPacket(packet, { "ephemeral-key": p384.privateKey }),
      /P-256/
    );
  });

  it("rejects absent, legacy, or ambiguous trust payloads", async () => {
    assert.throws(() => workerTrustPhase({} as WorkerTrustPacket), /payload/i);
    assert.throws(
      () => workerTrustPhase({ jobRequest: {} } as unknown as WorkerTrustPacket),
      /payload/i
    );
    const packet = await loadPacket("challenge");
    asRecord(packet).workerHandshakeResult = {};
    assert.throws(() => workerTrustPhase(packet), /payload/i);
  });
});
