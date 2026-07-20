import assert from "node:assert/strict";

import { loadRoot } from "../src/protos";
import { RuntimeWorkerTrust } from "../src/runtime-worker-trust";
import { signedDispatch } from "./runtime-trust-packet-support";
import { createTrustFixture } from "./worker-trust-runtime-support";

const silentLogger = { info() {}, warn() {}, error() {} };
const unusedRequester = {
  request: async () => { throw new Error("off mode must not request trust"); },
};

async function validPacket() {
  const root = await loadRoot();
  const type = root.lookupType("cordum.agent.v1.BusPacket");
  const packet = type.fromObject({
    traceId: "trace-off-reconnect",
    senderId: "scheduler-off",
    protocolVersion: 1,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobRequest: { jobId: "job-off", topic: "job.off" },
  });
  return { type, packet };
}

describe("RuntimeWorkerTrust coordinator", () => {
  it("keeps legacy off-mode admission open across reconnect", async () => {
    const trust = new RuntimeWorkerTrust({}, "worker-off", ["job.off"], silentLogger);
    const { type, packet } = await validPacket();
    await trust.authenticate(unusedRequester);
    assert.equal(trust.verifyInbound(type, packet, undefined), true);

    await trust.reauthenticate();

    assert.equal(trust.verifyInbound(type, packet, undefined), true);
  });

  it("does not mislabel ready topics as negotiation capabilities", () => {
    const trust = new RuntimeWorkerTrust({}, "worker-off", ["job.alpha"], silentLogger);

    assert.deepEqual(trust.capability.readyTopics, ["job.alpha"]);
    assert.deepEqual(trust.capability.capabilities, {});
  });

  it("does not admit after a warn-mode security response failure", async () => {
    const fixture = createTrustFixture();
    const trust = new RuntimeWorkerTrust({
      mode: "warn",
      config: fixture.config,
      timeoutMs: 50,
      retries: 3,
      renewMinIntervalMs: 1000,
    }, fixture.config.workerId, ["job.secure"], silentLogger);
    const requester = { request: async () => ({ data: new Uint8Array([255, 255]) }) };

    await assert.rejects(trust.authenticate(requester), /authenticated worker trust failed/);

    const root = await loadRoot();
    const type = root.lookupType("cordum.agent.v1.BusPacket");
    const packet = type.decode(await signedDispatch(
      fixture,
      "job.secure",
      "job-after-failure",
      "redis://ctx:after-failure"
    ));
    assert.equal(trust.verifyInbound(type, packet, undefined), false);
    await trust.close();
  });
});
