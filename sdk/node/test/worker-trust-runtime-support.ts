import crypto from "node:crypto";

import {
  WORKER_HANDSHAKE_AUDIENCE,
  createWorkerTrustConfig,
  marshalWorkerTrustPacket,
  timestampFromDate,
  unmarshalWorkerTrustPacket,
  type ChallengeRequestTrustPacket,
  type ChallengeTrustPacket,
  type ResultTrustPacket,
  type WorkerCapabilityHandshake,
  type WorkerTrustConfig,
  type WorkerTrustPacket,
} from "../src/worker-trust";
import { signWorkerTrustPacket, verifyWorkerTrustPacket } from "../src/trust-signing";
import {
  SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
  SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
} from "../src/protos";

const PROTOCOL_VERSION = 1;
const PROOF_ALGORITHM = 1;

export interface RequestRecord {
  readonly subject: string;
  readonly timeout: number | undefined;
  readonly packet: WorkerTrustPacket;
}

export interface TrustFixture {
  readonly config: WorkerTrustConfig;
  readonly workerPublicKey: crypto.KeyObject;
  readonly schedulerPrivateKey: crypto.KeyObject;
  readonly capability: WorkerCapabilityHandshake;
}

export function createTrustFixture(workerId = "worker-node"): TrustFixture {
  const worker = crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
  const scheduler = crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
  const config = createWorkerTrustConfig({
    workerId,
    expectedAgentId: "agent-node",
    tenantId: "tenant-node",
    audience: WORKER_HANDSHAKE_AUDIENCE,
    proofKeyId: "worker-key",
    proofPrivateKey: worker.privateKey,
    expectedSchedulerId: "scheduler-node",
    schedulerPublicKeys: { "scheduler-key": scheduler.publicKey },
    sdkVersion: "cap-node/v2",
  });
  return {
    config,
    workerPublicKey: worker.publicKey,
    schedulerPrivateKey: scheduler.privateKey,
    capability: {
      componentId: workerId,
      role: 3,
      capabilities: { "job.runtime.trust": true },
      readyTopics: ["job.runtime.trust"],
      supportedVersions: [1],
      sdkVersion: "cap-node/v2",
      agentName: "Node trust test",
    },
  };
}

function envelope(fixture: TrustFixture, traceId: string, now: Date) {
  return {
    traceId,
    senderId: fixture.config.expectedSchedulerId,
    protocolVersion: PROTOCOL_VERSION,
    createdAt: timestampFromDate(now),
    signature: new Uint8Array(),
  };
}

async function signedChallenge(
  fixture: TrustFixture,
  request: ChallengeRequestTrustPacket,
  now: Date
): Promise<WorkerTrustPacket> {
  const source = request.workerHandshakeChallengeRequest;
  const unsigned: ChallengeTrustPacket = {
    ...envelope(fixture, source.traceId, now),
    workerHandshakeChallenge: {
      requestId: source.requestId,
      challengeId: `challenge-${source.requestId}`,
      traceId: source.traceId,
      workerId: source.workerId,
      agentId: fixture.config.expectedAgentId,
      tenantId: fixture.config.tenantId,
      proofKeyId: source.proofKeyId,
      proofAlgorithm: PROOF_ALGORITHM,
      serverKeyId: "scheduler-key",
      audience: source.audience,
      purpose: source.purpose,
      clientNonce: source.clientNonce,
      serverNonce: new Uint8Array(32).fill(83),
      issuedAt: timestampFromDate(now),
      expiresAt: timestampFromDate(new Date(now.getTime() + 30_000)),
      protocolVersion: PROTOCOL_VERSION,
      sdkVersion: source.sdkVersion,
    },
  };
  return signWorkerTrustPacket(unsigned, { "scheduler-key": fixture.schedulerPrivateKey });
}

async function signedResult(
  fixture: TrustFixture,
  request: WorkerTrustPacket,
  now: Date,
  token: string
): Promise<ResultTrustPacket> {
  if (!("workerHandshakeAuthenticate" in request)) throw new Error("authenticate expected");
  const challenge = request.workerHandshakeAuthenticate.challenge;
  const unsigned: ResultTrustPacket = {
    ...envelope(fixture, challenge.traceId, now),
    authToken: token,
    workerHandshakeResult: {
      challenge,
      accepted: true,
      rejectionReason: 0,
      issuedAt: timestampFromDate(now),
      tokenExpiresAt: timestampFromDate(new Date(now.getTime() + 300_000)),
    },
  };
  return signWorkerTrustPacket(unsigned, { "scheduler-key": fixture.schedulerPrivateKey });
}

export class FakeTrustRequester {
  readonly requests: RequestRecord[] = [];
  private tokenNumber = 0;

  constructor(
    readonly fixture: TrustFixture,
    private readonly clock: () => Date = () => new Date()
  ) {}

  async request(
    subject: string,
    data: Uint8Array,
    options?: { timeout?: number }
  ): Promise<{ data: Uint8Array }> {
    const packet = await unmarshalWorkerTrustPacket(data);
    this.requests.push({ subject, timeout: options?.timeout, packet });
    const valid = await verifyWorkerTrustPacket(packet, {
      [this.fixture.config.proofKeyId]: this.fixture.workerPublicKey,
    });
    if (!valid) throw new Error("worker request signature is invalid");
    const response = await this.response(subject, packet);
    return { data: await marshalWorkerTrustPacket(response) };
  }

  protected async response(subject: string, packet: WorkerTrustPacket) {
    const now = this.clock();
    if (subject === SUBJECT_WORKER_HANDSHAKE_CHALLENGE) {
      if (!("workerHandshakeChallengeRequest" in packet)) throw new Error("challenge expected");
      return signedChallenge(this.fixture, packet, now);
    }
    if (subject === SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE) {
      this.tokenNumber += 1;
      return signedResult(this.fixture, packet, now, `session-${this.tokenNumber}`);
    }
    throw new Error(`unexpected trust subject: ${subject}`);
  }
}
