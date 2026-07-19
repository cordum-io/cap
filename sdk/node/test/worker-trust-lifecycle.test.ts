import assert from "node:assert/strict";

import {
  WorkerTrustOperationalError,
  WorkerTrustLifecycle,
  type WorkerTrustExchange,
} from "../src/worker-trust-lifecycle";
import type {
  WorkerHandshakePurpose,
  WorkerHandshakeSession,
} from "../src/worker-trust-contract";
import {
  WorkerHandshakePacketError,
  WorkerHandshakeRejectionError,
} from "../src/worker-trust-contract";
import type { RuntimeTrustSettings } from "../src/worker-trust-runtime-config";

const silentLogger = { info() {}, warn() {}, error() {} };

interface ExchangeCall {
  purpose: WorkerHandshakePurpose;
  token: string;
}

class QueueExchange implements WorkerTrustExchange {
  readonly calls: ExchangeCall[] = [];

  constructor(private readonly outcomes: Array<WorkerHandshakeSession | Error>) {}

  async exchange(
    purpose: WorkerHandshakePurpose,
    currentToken: string
  ): Promise<WorkerHandshakeSession> {
    this.calls.push({ purpose, token: currentToken });
    const outcome = this.outcomes.shift();
    if (!outcome) throw new Error("no queued trust outcome");
    if (outcome instanceof Error) throw outcome;
    return outcome;
  }
}

function settings(
  mode: "warn" | "enforce",
  renewMinIntervalMs = 5,
  retries = 1
): RuntimeTrustSettings {
  return {
    mode,
    config: undefined,
    timeoutMs: 50,
    retries,
    renewMinIntervalMs,
  };
}

function session(token: string, lifetimeMs = 100): WorkerHandshakeSession {
  const issuedAt = new Date();
  return { token, issuedAt, expiresAt: new Date(issuedAt.getTime() + lifetimeMs) };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

async function waitFor(predicate: () => boolean, timeoutMs = 500): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (!predicate()) {
    if (Date.now() >= deadline) throw new Error("condition was not met");
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
}

describe("WorkerTrustLifecycle", () => {
  it("installs and atomically rotates accepted session tokens", async () => {
    const exchange = new QueueExchange([session("token-one"), session("token-two")]);
    const lifecycle = new WorkerTrustLifecycle(exchange, settings("enforce"), silentLogger);

    assert.equal(await lifecycle.authenticate(), true);
    assert.equal(lifecycle.sessionToken, "token-one");
    assert.equal(await lifecycle.renew(), true);
    assert.equal(lifecycle.sessionToken, "token-two");
    assert.deepEqual(exchange.calls, [
      { purpose: 1, token: "" },
      { purpose: 2, token: "token-one" },
    ]);
  });

  it("warn mode proceeds tokenless after a bounded exchange failure", async () => {
    const exchange = new QueueExchange([
      new WorkerTrustOperationalError(new Error("scheduler unavailable")),
    ]);
    const lifecycle = new WorkerTrustLifecycle(exchange, settings("warn"), silentLogger);

    assert.equal(await lifecycle.authenticate(), false);
    assert.equal(lifecycle.sessionToken, undefined);
  });

  it("retries only an operational transport failure", async () => {
    const exchange = new QueueExchange([
      new WorkerTrustOperationalError(new Error("scheduler unavailable")),
      session("recovered"),
    ]);
    const lifecycle = new WorkerTrustLifecycle(
      exchange,
      settings("warn", 5, 2),
      silentLogger
    );

    assert.equal(await lifecycle.authenticate(), true);
    assert.equal(lifecycle.sessionToken, "recovered");
    assert.equal(exchange.calls.length, 2);
  });

  it("warn mode fails closed on an authenticated scheduler rejection", async () => {
    const exchange = new QueueExchange([new WorkerHandshakeRejectionError(1)]);
    const lifecycle = new WorkerTrustLifecycle(exchange, settings("warn"), silentLogger);

    await assert.rejects(lifecycle.authenticate(), /authenticated worker trust failed/);
    assert.equal(lifecycle.sessionToken, undefined);
  });

  it("warn mode fails closed on a tampered handshake packet", async () => {
    const exchange = new QueueExchange([new WorkerHandshakePacketError("tampered packet")]);
    const lifecycle = new WorkerTrustLifecycle(
      exchange,
      settings("warn", 5, 3),
      silentLogger
    );

    await assert.rejects(lifecycle.authenticate(), /authenticated worker trust failed/);
    assert.equal(lifecycle.sessionToken, undefined);
    assert.equal(exchange.calls.length, 1);
  });

  it("enforce mode fails closed after an exchange failure", async () => {
    const exchange = new QueueExchange([new Error("scheduler unavailable")]);
    const lifecycle = new WorkerTrustLifecycle(exchange, settings("enforce"), silentLogger);

    await assert.rejects(lifecycle.authenticate(), /authenticated worker trust failed/);
    assert.equal(lifecycle.sessionToken, undefined);
  });

  it("does not retry an authenticated scheduler rejection", async () => {
    const exchange = new QueueExchange([
      new WorkerHandshakeRejectionError(1),
      session("must-not-install"),
    ]);
    const lifecycle = new WorkerTrustLifecycle(
      exchange,
      settings("enforce", 5, 3),
      silentLogger
    );

    await assert.rejects(lifecycle.authenticate(), /authenticated worker trust failed/);
    assert.equal(exchange.calls.length, 1);
  });

  it("never installs a session that completes after close", async () => {
    const result = deferred<WorkerHandshakeSession>();
    const exchange: WorkerTrustExchange = { exchange: async () => result.promise };
    const lifecycle = new WorkerTrustLifecycle(exchange, settings("enforce"), silentLogger);
    const pending = lifecycle.authenticate();

    await lifecycle.close();
    result.resolve(session("late-token"));

    await assert.rejects(pending, /closed/);
    assert.equal(lifecycle.sessionToken, undefined);
  });

  it("renews before expiry and rotates the token used by callers", async () => {
    const exchange = new QueueExchange([session("first", 80), session("second", 1000)]);
    const lifecycle = new WorkerTrustLifecycle(exchange, settings("enforce"), silentLogger);
    await lifecycle.authenticate();

    lifecycle.startRenewal(async () => undefined);
    await waitFor(() => lifecycle.sessionToken === "second");

    assert.equal(exchange.calls[1]?.token, "first");
    await lifecycle.close();
  });

  it("clears enforce admission and reports a renewal failure once", async () => {
    const exchange = new QueueExchange([session("first", 50), new Error("renew failed")]);
    const lifecycle = new WorkerTrustLifecycle(exchange, settings("enforce"), silentLogger);
    const failures: Error[] = [];
    await lifecycle.authenticate();

    lifecycle.startRenewal(async (error: Error) => { failures.push(error); });
    await waitFor(() => failures.length === 1);

    assert.equal(lifecycle.sessionToken, undefined);
    assert.equal(failures.length, 1);
    await lifecycle.close();
  });

  it("close cancels a hung renewal without reporting a trust outage", async () => {
    const pending = deferred<WorkerHandshakeSession>();
    let calls = 0;
    const exchange: WorkerTrustExchange = {
      exchange: async () => {
        calls += 1;
        return calls === 1 ? session("first", 40) : pending.promise;
      },
    };
    const lifecycle = new WorkerTrustLifecycle(exchange, settings("enforce"), silentLogger);
    let failures = 0;
    await lifecycle.authenticate();
    lifecycle.startRenewal(async () => { failures += 1; });
    await waitFor(() => calls === 2);

    await lifecycle.close();

    assert.equal(failures, 0);
    assert.equal(lifecycle.renewalRunning, false);
  });

  it("close cancels overlapping renewal and reconnect retry timers", async () => {
    let calls = 0;
    const exchange: WorkerTrustExchange = {
      exchange: async () => {
        calls += 1;
        if (calls === 1) return session("first", 300_000);
        throw new Error("reconnect retry");
      },
    };
    const lifecycle = new WorkerTrustLifecycle(
      exchange,
      settings("enforce", 5, 2),
      silentLogger
    );
    await lifecycle.authenticate();
    lifecycle.startRenewal(async () => undefined);
    const reconnect = lifecycle.reauthenticate().catch(() => false);
    await waitFor(() => calls === 2);

    const closedQuickly = await Promise.race([
      lifecycle.close().then(() => true),
      new Promise<boolean>((resolve) => setTimeout(() => resolve(false), 100)),
    ]);

    assert.equal(closedQuickly, true);
    await reconnect;
  });
});
