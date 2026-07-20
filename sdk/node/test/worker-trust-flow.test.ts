import * as assert from "node:assert";
import * as crypto from "node:crypto";

import {
  WORKER_HANDSHAKE_AUDIENCE,
  AuthenticateTrustPacket,
  ChallengeRequestTrustPacket,
  ChallengeTrustPacket,
  ResultTrustPacket,
  VerifiedWorkerHandshakeChallenge,
  WorkerHandshakeChallenge,
  WorkerTrustConfig,
  buildAuthenticate,
  buildChallengeRequest,
  createWorkerTrustConfig,
  timestampFromDate,
  validateWorkerTrustPacket,
  verifyChallenge,
  verifyResult,
} from "../src/worker-trust";
import { signWorkerTrustPacket, verifyWorkerTrustPacket } from "../src/trust-signing";

const NOW = new Date("2026-07-19T00:00:00.000Z");

interface Fixture {
  config: WorkerTrustConfig;
  worker: crypto.KeyPairKeyObjectResult;
  scheduler: crypto.KeyPairKeyObjectResult;
}

function fixture(): Fixture {
  const worker = crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
  const scheduler = crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
  const config = createWorkerTrustConfig({
    workerId: "worker-node",
    expectedAgentId: "agent-node",
    tenantId: "tenant-node",
    audience: WORKER_HANDSHAKE_AUDIENCE,
    proofKeyId: "worker-key-node",
    proofPrivateKey: worker.privateKey,
    expectedSchedulerId: "scheduler-node",
    schedulerPublicKeys: { "scheduler-key-node": scheduler.publicKey },
    sdkVersion: "cap-node/test",
  });
  return { config, worker, scheduler };
}

function options(purpose: 1 | 2 = 1) {
  return {
    requestId: "request-node",
    traceId: "trace-node",
    purpose,
    clientNonce: new Uint8Array(32).fill(0x21),
    createdAt: NOW,
  } as const;
}

function capability() {
  return {
    componentId: "worker-node",
    role: 3,
    supportedVersions: [1],
    capabilities: { progress: true },
    sdkVersion: "cap-node/test",
    readyTopics: ["jobs.node"],
    agentName: "Node Worker",
  };
}

async function signedChallenge(
  f: Fixture,
  request: ChallengeRequestTrustPacket,
  mutate?: (challenge: WorkerHandshakeChallenge) => void
): Promise<ChallengeTrustPacket> {
  const source = request.workerHandshakeChallengeRequest;
  const challenge: WorkerHandshakeChallenge = {
    ...source,
    challengeId: "challenge-node",
    agentId: f.config.expectedAgentId,
    tenantId: f.config.tenantId,
    serverKeyId: "scheduler-key-node",
    serverNonce: new Uint8Array(32).fill(0x32),
    issuedAt: timestampFromDate(NOW),
    expiresAt: timestampFromDate(new Date(NOW.getTime() + 30_000)),
  };
  mutate?.(challenge);
  return signWorkerTrustPacket({
    traceId: challenge.traceId,
    senderId: f.config.expectedSchedulerId,
    createdAt: timestampFromDate(NOW),
    protocolVersion: 1,
    signature: new Uint8Array(),
    workerHandshakeChallenge: challenge,
  }, { "scheduler-key-node": f.scheduler.privateKey });
}

async function signedResult(
  f: Fixture,
  verified: VerifiedWorkerHandshakeChallenge,
  token = "session-node-b",
  mutate?: (packet: ResultTrustPacket) => void
): Promise<ResultTrustPacket> {
  const packet: ResultTrustPacket = {
    traceId: verified.message().traceId,
    senderId: f.config.expectedSchedulerId,
    createdAt: timestampFromDate(NOW),
    protocolVersion: 1,
    signature: new Uint8Array(),
    authToken: token,
    workerHandshakeResult: {
      challenge: verified.message(),
      accepted: true,
      rejectionReason: 0,
      issuedAt: timestampFromDate(NOW),
      tokenExpiresAt: timestampFromDate(new Date(NOW.getTime() + 45_000)),
    },
  };
  mutate?.(packet);
  return signWorkerTrustPacket(packet, { "scheduler-key-node": f.scheduler.privateKey });
}

describe("worker trust client flow", () => {
  it("builds self-validating signed request and authenticate packets", async () => {
    const f = fixture();
    const request = await buildChallengeRequest(f.config, options());
    assert.doesNotThrow(() => validateWorkerTrustPacket(request));
    assert.strictEqual(
      await verifyWorkerTrustPacket(request, { [f.config.proofKeyId]: f.worker.publicKey }),
      true
    );
    const verified = await verifyChallenge(f.config, request, await signedChallenge(f, request), NOW);
    const authenticate = await buildAuthenticate(f.config, verified, capability(), "", NOW);
    assert.doesNotThrow(() => validateWorkerTrustPacket(authenticate));
    assert.strictEqual(
      await verifyWorkerTrustPacket(authenticate, { [f.config.proofKeyId]: f.worker.publicKey }),
      true
    );
  });

  it("installs only a signed live correlated result", async () => {
    const f = fixture();
    const request = await buildChallengeRequest(f.config, options());
    const verified = await verifyChallenge(f.config, request, await signedChallenge(f, request), NOW);
    const authenticate = await buildAuthenticate(f.config, verified, capability(), "", NOW);
    const session = await verifyResult(
      f.config, verified, authenticate, await signedResult(f, verified), NOW
    );
    assert.deepStrictEqual(session, {
      token: "session-node-b",
      issuedAt: NOW,
      expiresAt: new Date(NOW.getTime() + 45_000),
    });
  });
});

describe("worker trust challenge rejection", () => {
  it("rejects signed challenge identity, correlation, audience, and freshness changes", async () => {
    const mutations: Array<(challenge: WorkerHandshakeChallenge) => void> = [
      (value) => { value.requestId = "request-other"; },
      (value) => { value.traceId = "trace-other"; },
      (value) => { value.workerId = "worker-other"; },
      (value) => { value.agentId = "agent-other"; },
      (value) => { value.tenantId = "tenant-other"; },
      (value) => { value.audience = "other"; },
      (value) => { value.sdkVersion = "other"; },
      (value) => { value.clientNonce = new Uint8Array(32).fill(0x99); },
      (value) => { value.expiresAt = timestampFromDate(NOW); },
      (value) => {
        value.expiresAt = {
          ...timestampFromDate(new Date(NOW.getTime() + 60_000)),
          nanos: 1,
        };
      },
      (value) => {
        value.issuedAt = {
          ...timestampFromDate(new Date(NOW.getTime() + 60_000)),
          nanos: 1,
        };
        value.expiresAt = timestampFromDate(new Date(NOW.getTime() + 61_000));
      },
    ];
    for (const mutate of mutations) {
      const f = fixture();
      const request = await buildChallengeRequest(f.config, options());
      await assert.rejects(
        () => verifyChallenge(f.config, request, signedChallenge(f, request, mutate), NOW),
        /binding|correlation|identity|expired|lifetime|skew/i
      );
    }
  });

  it("rejects tamper, forged verification handles, and non-rotating renewal", async () => {
    const f = fixture();
    const request = await buildChallengeRequest(f.config, options());
    const challenge = await signedChallenge(f, request);
    challenge.workerHandshakeChallenge.agentId = "tampered";
    await assert.rejects(() => verifyChallenge(f.config, request, challenge, NOW), /signature/i);

    const fresh = await signedChallenge(f, request);
    const fake = { request, response: fresh, message: () => fresh.workerHandshakeChallenge } as
      unknown as VerifiedWorkerHandshakeChallenge;
    await assert.rejects(
      () => buildAuthenticate(f.config, fake, capability(), "", NOW),
      /verified challenge/i
    );

    const renewRequest = await buildChallengeRequest(f.config, options(2));
    const renew = await verifyChallenge(f.config, renewRequest, await signedChallenge(f, renewRequest), NOW);
    const authenticate = await buildAuthenticate(f.config, renew, capability(), "session-node-a", NOW);
    await assert.rejects(
      () => verifyResult(f.config, renew, authenticate, signedResult(f, renew, "session-node-a"), NOW),
      /rotate/i
    );
  });
});

describe("worker trust configured identity", () => {
  it("binds a valid worker signature to the configured worker identity", async () => {
    const f = fixture();
    const original = await buildChallengeRequest(f.config, options());
    original.senderId = "victim-worker";
    original.workerHandshakeChallengeRequest.workerId = "victim-worker";
    original.workerHandshakeChallengeRequest.audience = "other-scheduler";
    const impersonation = await signWorkerTrustPacket(original, {
      [f.config.proofKeyId]: f.worker.privateKey,
    });
    await assert.rejects(
      () => verifyChallenge(
        f.config, impersonation, signedChallenge(f, impersonation), NOW
      ),
      /configured worker identity/i
    );
  });
});

describe("worker trust result rejection", () => {
  it("rejects signed result correlation, identity, token, and timing mismatches", async () => {
    const f = fixture();
    const request = await buildChallengeRequest(f.config, options());
    const verified = await verifyChallenge(f.config, request, await signedChallenge(f, request), NOW);
    const authenticate = await buildAuthenticate(f.config, verified, capability(), "", NOW);
    const mutations: Array<(packet: ResultTrustPacket) => void> = [
      (packet) => { packet.traceId = "trace-other"; },
      (packet) => { packet.workerHandshakeResult.challenge.requestId = "request-other"; },
      (packet) => { packet.workerHandshakeResult.challenge.challengeId = "challenge-other"; },
      (packet) => { packet.workerHandshakeResult.challenge.workerId = "worker-other"; },
      (packet) => { packet.workerHandshakeResult.challenge.agentId = "agent-other"; },
      (packet) => { packet.workerHandshakeResult.challenge.tenantId = "tenant-other"; },
      (packet) => { packet.workerHandshakeResult.challenge.audience = "other"; },
      (packet) => { packet.workerHandshakeResult.challenge.issuedAt.nanos = 1; },
      (packet) => {
        packet.workerHandshakeResult.issuedAt = timestampFromDate(
          new Date(NOW.getTime() + 61_000)
        );
        packet.workerHandshakeResult.tokenExpiresAt = timestampFromDate(
          new Date(NOW.getTime() + 62_000)
        );
      },
    ];
    for (const mutate of mutations) {
      await assert.rejects(
        () => verifyResult(
          f.config, verified, authenticate, signedResult(f, verified, "session-node-b", mutate), NOW
        ),
        /binding|correlation|identity|trace|skew/i
      );
    }
    await assert.rejects(
      () => verifyResult(f.config, verified, authenticate, signedResult(f, verified, ""), NOW),
      /token/i
    );
  });
});
