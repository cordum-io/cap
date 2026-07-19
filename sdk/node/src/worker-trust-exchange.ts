import { randomBytes, randomUUID } from "node:crypto";
import { ErrorCode, NatsError } from "nats";

import {
  buildAuthenticate,
  buildChallengeRequest,
  marshalWorkerTrustPacket,
  unmarshalWorkerTrustPacket,
  verifyChallenge,
  verifyResult,
  WORKER_HANDSHAKE_NONCE_SIZE,
  type ChallengeTrustPacket,
  type ResultTrustPacket,
  type WorkerCapabilityHandshake,
  type WorkerHandshakePurpose,
  type WorkerHandshakeSession,
  type WorkerTrustConfig,
  type WorkerTrustPacket,
} from "./worker-trust";
import {
  SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
  SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
} from "./protos";
import type { RuntimeTrustSettings } from "./worker-trust-runtime-config";
import {
  WorkerTrustOperationalError,
  type WorkerTrustExchange,
} from "./worker-trust-lifecycle";

const OPERATIONAL_NATS_CODES: ReadonlySet<string> = new Set([
  ErrorCode.Cancelled,
  ErrorCode.ConnectionClosed,
  ErrorCode.ConnectionDraining,
  ErrorCode.ConnectionRefused,
  ErrorCode.ConnectionTimeout,
  ErrorCode.Disconnect,
  ErrorCode.NoResponders,
  ErrorCode.SubClosed,
  ErrorCode.SubDraining,
  ErrorCode.Timeout,
]);

export interface WorkerTrustRequester {
  request(
    subject: string,
    data: Uint8Array,
    options: { timeout: number }
  ): Promise<{ data: Uint8Array }>;
}

function requestId(): string {
  return randomUUID().replace(/-/g, "");
}

function requireChallenge(packet: WorkerTrustPacket): ChallengeTrustPacket {
  if (!("workerHandshakeChallenge" in packet)) {
    throw new Error("worker trust response is not a challenge");
  }
  return packet;
}

function requireResult(packet: WorkerTrustPacket): ResultTrustPacket {
  if (!("workerHandshakeResult" in packet)) {
    throw new Error("worker trust response is not a result");
  }
  return packet;
}

function classifyRequestError(error: unknown): WorkerTrustOperationalError | undefined {
  if (error instanceof NatsError && OPERATIONAL_NATS_CODES.has(error.code)) {
    return new WorkerTrustOperationalError(error);
  }
  return undefined;
}

export class NatsWorkerTrustExchange implements WorkerTrustExchange {
  private readonly config: WorkerTrustConfig;

  constructor(
    private readonly requester: WorkerTrustRequester,
    private readonly settings: RuntimeTrustSettings,
    private readonly capability: WorkerCapabilityHandshake,
    private readonly clock: () => Date = () => new Date()
  ) {
    if (!settings.config) throw new Error("worker trust configuration is required");
    this.config = settings.config;
  }

  async exchange(
    purpose: WorkerHandshakePurpose,
    currentToken: string
  ): Promise<WorkerHandshakeSession> {
    const createdAt = this.clock();
    const request = await buildChallengeRequest(this.config, {
      requestId: requestId(),
      traceId: requestId(),
      purpose,
      clientNonce: randomBytes(WORKER_HANDSHAKE_NONCE_SIZE),
      createdAt,
    });
    const challengePacket = requireChallenge(await this.requestPacket(
      SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
      request
    ));
    const verified = await verifyChallenge(
      this.config,
      request,
      challengePacket,
      this.clock()
    );
    const authenticate = await buildAuthenticate(
      this.config,
      verified,
      this.capability,
      currentToken,
      this.clock()
    );
    const resultPacket = requireResult(await this.requestPacket(
      SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
      authenticate
    ));
    return verifyResult(
      this.config,
      verified,
      authenticate,
      resultPacket,
      this.clock()
    );
  }

  private async requestPacket(
    subject: string,
    packet: WorkerTrustPacket
  ): Promise<WorkerTrustPacket> {
    const data = await marshalWorkerTrustPacket(packet);
    let response: { data: Uint8Array };
    try {
      response = await this.requester.request(subject, data, {
        timeout: this.settings.timeoutMs,
      });
    } catch (error) {
      const operational = classifyRequestError(error);
      if (operational) throw operational;
      throw error;
    }
    return unmarshalWorkerTrustPacket(response.data);
  }
}
