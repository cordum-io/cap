import assert from "node:assert/strict";
import { ErrorCode, NatsError } from "nats";

import { NatsWorkerTrustExchange } from "../src/worker-trust-exchange";
import { WorkerTrustOperationalError } from "../src/worker-trust-lifecycle";
import type { RuntimeTrustSettings } from "../src/worker-trust-runtime-config";
import {
  SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
  SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
} from "../src/protos";
import {
  FakeTrustRequester,
  createTrustFixture,
} from "./worker-trust-runtime-support";

describe("NatsWorkerTrustExchange", () => {
  it("completes the signed challenge and authenticate requests in order", async () => {
    const fixture = createTrustFixture();
    const requester = new FakeTrustRequester(fixture);
    const settings: RuntimeTrustSettings = {
      mode: "enforce",
      config: fixture.config,
      timeoutMs: 75,
      retries: 1,
      renewMinIntervalMs: 1000,
    };
    const exchange = new NatsWorkerTrustExchange(requester, settings, fixture.capability);

    const issued = await exchange.exchange(1, "");
    const renewed = await exchange.exchange(2, issued.token);

    assert.equal(issued.token, "session-1");
    assert.equal(renewed.token, "session-2");
    assert.deepEqual(
      requester.requests.map(({ subject }) => subject),
      [
        SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
        SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
        SUBJECT_WORKER_HANDSHAKE_CHALLENGE,
        SUBJECT_WORKER_HANDSHAKE_AUTHENTICATE,
      ]
    );
    assert.deepEqual(requester.requests.map(({ timeout }) => timeout), [75, 75, 75, 75]);
    const firstAuth = requester.requests[1]?.packet;
    const renewAuth = requester.requests[3]?.packet;
    assert.equal(firstAuth?.authToken, "");
    assert.equal(renewAuth?.authToken, "session-1");
  });

  it("rejects a malformed response before a session can be installed", async () => {
    const fixture = createTrustFixture();
    const requester = {
      request: async () => ({ data: new Uint8Array([255, 255, 255]) }),
    };
    const settings: RuntimeTrustSettings = {
      mode: "enforce",
      config: fixture.config,
      timeoutMs: 75,
      retries: 1,
      renewMinIntervalMs: 1000,
    };
    const exchange = new NatsWorkerTrustExchange(requester, settings, fixture.capability);

    await assert.rejects(exchange.exchange(1, ""), /worker (handshake|trust) packet|decode|wire/i);
  });

  it("classifies allowlisted NATS unavailability without leaking its message", async () => {
    const fixture = createTrustFixture();
    const unavailable = new NatsError("private scheduler route", ErrorCode.NoResponders);
    const requester = { request: async () => { throw unavailable; } };
    const settings: RuntimeTrustSettings = {
      mode: "warn",
      config: fixture.config,
      timeoutMs: 75,
      retries: 1,
      renewMinIntervalMs: 1000,
    };
    const exchange = new NatsWorkerTrustExchange(requester, settings, fixture.capability);

    await assert.rejects(exchange.exchange(1, ""), (error: unknown) => {
      assert.ok(error instanceof WorkerTrustOperationalError);
      assert.equal(error.message, "worker trust transport is unavailable");
      assert.equal(error.message.includes("private scheduler route"), false);
      return true;
    });
  });

  it("does not classify permission failures as operational unavailability", async () => {
    const fixture = createTrustFixture();
    const denied = new NatsError("permission denied", ErrorCode.PermissionsViolation);
    const requester = { request: async () => { throw denied; } };
    const settings: RuntimeTrustSettings = {
      mode: "warn",
      config: fixture.config,
      timeoutMs: 75,
      retries: 1,
      renewMinIntervalMs: 1000,
    };
    const exchange = new NatsWorkerTrustExchange(requester, settings, fixture.capability);

    await assert.rejects(exchange.exchange(1, ""), (error: unknown) => {
      assert.equal(error, denied);
      assert.equal(error instanceof WorkerTrustOperationalError, false);
      return true;
    });
  });
});
