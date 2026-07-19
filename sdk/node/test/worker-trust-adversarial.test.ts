import assert from "node:assert/strict";

import {
  buildAuthenticate,
  buildChallengeRequest,
  marshalWorkerTrustPacket,
  timestampFromDate,
  unmarshalWorkerTrustPacket,
  verifyChallenge,
  verifyResult,
  type ChallengeTrustPacket,
  type ResultTrustPacket,
  type VerifiedWorkerHandshakeChallenge,
} from "../src/worker-trust";
import { SUBJECT_WORKER_HANDSHAKE_CHALLENGE } from "../src/protos";
import { signWorkerTrustPacket } from "../src/trust-signing";
import {
  FakeTrustRequester,
  createTrustFixture,
  type TrustFixture,
} from "./worker-trust-runtime-support";

const NOW = new Date("2026-07-19T00:00:00.000Z");

async function challengeFlow(clock: () => Date = () => NOW) {
  const fixture = createTrustFixture();
  const requester = new FakeTrustRequester(fixture, clock);
  const request = await buildChallengeRequest(fixture.config, {
    requestId: "request-adversarial",
    traceId: "trace-adversarial",
    purpose: 1,
    clientNonce: new Uint8Array(32).fill(0x42),
    createdAt: clock(),
  });
  const reply = await requester.request(
    SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
    await marshalWorkerTrustPacket(request)
  );
  const challenge = await unmarshalWorkerTrustPacket(reply.data) as ChallengeTrustPacket;
  const verified = await verifyChallenge(fixture.config, request, challenge, clock());
  return { fixture, requester, request, verified };
}

async function resignResult(
  fixture: TrustFixture,
  packet: ResultTrustPacket
): Promise<ResultTrustPacket> {
  packet.signature = new Uint8Array();
  return signWorkerTrustPacket(packet, { "scheduler-key": fixture.schedulerPrivateKey });
}

describe("worker trust adversarial boundaries", () => {
  it("does not expose verifier authority through a mutable prototype", async () => {
    const { fixture, verified } = await challengeFlow();
    const prototype = Object.getPrototypeOf(verified) as Record<string, unknown>;
    const original = Object.getOwnPropertyDescriptor(prototype, "packets");
    let captured: symbol | undefined;
    Object.defineProperty(prototype, "packets", {
      configurable: true,
      value(secret: symbol) {
        captured = secret;
        throw new Error("prototype hook invoked");
      },
    });
    try {
      const packet = await buildAuthenticate(
        fixture.config, verified, fixture.capability, "", NOW
      );
      assert.equal(packet.senderId, fixture.config.workerId);
      assert.equal(captured, undefined);
    } finally {
      if (original) Object.defineProperty(prototype, "packets", original);
      else delete prototype.packets;
    }
  });

  it("rejects a locally signed request outside the allowed clock skew", async () => {
    const fixture = createTrustFixture();
    const requester = new FakeTrustRequester(fixture, () => NOW);
    const request = await buildChallengeRequest(fixture.config, {
      requestId: "request-stale",
      traceId: "trace-stale",
      purpose: 1,
      clientNonce: new Uint8Array(32).fill(0x43),
      createdAt: new Date(NOW.getTime() - 60_001),
    });
    const reply = await requester.request(
      SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
      await marshalWorkerTrustPacket(request)
    );
    const challenge = await unmarshalWorkerTrustPacket(reply.data) as ChallengeTrustPacket;
    await assert.rejects(
      verifyChallenge(fixture.config, request, challenge, NOW),
      /request created_at.*skew/i
    );
  });

  it("rechecks challenge expiry when accepting the result", async () => {
    let now = NOW;
    const flow = await challengeFlow(() => now);
    const authenticate = await buildAuthenticate(
      flow.fixture.config, flow.verified, flow.fixture.capability, "", now
    );
    now = new Date(NOW.getTime() + 30_001);
    const response = await flow.requester.request(
      "sys.worker.handshake.authenticate",
      await marshalWorkerTrustPacket(authenticate)
    );
    const result = await unmarshalWorkerTrustPacket(response.data) as ResultTrustPacket;
    await assert.rejects(
      verifyResult(flow.fixture.config, flow.verified, authenticate, result, now),
      /challenge.*expired/i
    );
  });

  it("rejects a non-ASCII session token before signing or acceptance", async () => {
    const flow = await challengeFlow();
    const authenticate = await buildAuthenticate(
      flow.fixture.config, flow.verified, flow.fixture.capability, "", NOW
    );
    const response = await flow.requester.request(
      "sys.worker.handshake.authenticate",
      await marshalWorkerTrustPacket(authenticate)
    );
    const result = await unmarshalWorkerTrustPacket(response.data) as ResultTrustPacket;
    result.authToken = "session-\u0000-node";
    await assert.rejects(resignResult(flow.fixture, result), /token/i);
  });
});
