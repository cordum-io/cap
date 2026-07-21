import { expect } from "chai";
import type { Type } from "protobufjs";

import {
  freezeProductionAuthority,
  type ProductionEventAuthority,
  type ProductionIdentityView,
  type ProductionDispatchView,
} from "../src/production-events";
import { verifyProductionPacket } from "../src/production-signing";
import { Agent, InMemoryBlobStore } from "../src/runtime";
import { SUBJECT_RESULT } from "../src/protos";
import {
  busPacketType,
  keyPair,
  RecordingReplayStore,
  signedRawPacket,
} from "./production-admission-support";

interface ProductionRequest {
  jobId: string;
  identity?: ProductionIdentityView;
  dispatch?: ProductionDispatchView;
}

interface ProductionRuntimeHarness {
  busPacketType: Type;
  jobResultType: Type;
  nc: { publish(subject: string, data: Uint8Array): void };
  trust?: {
    productionSessionActive?: boolean;
    outboundSessionToken(): string;
    settings: {
      config: {
        proofKeyId: string;
        proofPrivateKey: string;
        tenantId?: string;
        expectedSchedulerId?: string;
      };
    };
  };
  publishResult(
    context: { packet: { traceId: string }; log: Console },
    request: ProductionRequest,
    resultPtr: string,
    executionMs: number,
    overrides?: Record<string, unknown>,
    authority?: ProductionEventAuthority,
  ): Promise<void>;
  onMessage(
    message: { data: Uint8Array; subject: string },
    spec: {
      topic: string;
      handler(context: { job: ProductionRequest }): Promise<Record<string, unknown>>;
      retries: number;
    },
  ): Promise<void>;
}

interface VerifiedResultPacket {
  jobResult: {
    identity: ProductionIdentityView;
    dispatch: ProductionDispatchView;
  };
  identity: ProductionIdentityView;
  authToken: string;
  signatureMetadata: { messageId: Uint8Array };
}

function productionRuntime(agent: Agent): ProductionRuntimeHarness {
  return agent as unknown as ProductionRuntimeHarness;
}

describe("production runtime worker echo", () => {
  it("signs automatic results with frozen identity and dispatch", async () => {
    const type = await busPacketType();
    const jobResultType = type.root.lookupType("cordum.agent.v1.JobResult");
    const published: Array<{ subject: string; data: Buffer }> = [];
    const agent = new Agent({
      senderId: "worker-a",
      store: new InMemoryBlobStore(),
      productionTrust: {
        audience: "job.worker.pool-a",
        tenant: "tenant-a",
        sender: "scheduler-a",
        publicKeys: { "scheduler-key": keyPair.publicKey },
      },
      replayStore: new RecordingReplayStore(),
    });
    const runtime = productionRuntime(agent);
    runtime.busPacketType = type;
    runtime.jobResultType = jobResultType;
    runtime.nc = {
      publish(subject: string, data: Uint8Array) {
        published.push({ subject, data: Buffer.from(data) });
      },
    };
    runtime.trust = {
      outboundSessionToken: () => "session-a",
      settings: {
        config: {
          proofKeyId: "worker-key",
          proofPrivateKey: keyPair.privateKey,
        },
      },
    };
    const request = {
      jobId: "job-a",
      identity: { tenantId: "tenant-a", principalId: "principal-a", actorId: "actor-a" },
      dispatch: { dispatchId: "dispatch-a", attempt: 3, assignedWorkerId: "worker-a" },
    };
    const authority = freezeProductionAuthority(request);
    const context = {
      packet: { traceId: "trace-a" },
      log: console,
    };

    await runtime.publishResult(context, request, "redis://result-a", 5, {}, authority);
    await runtime.publishResult(context, request, "redis://result-b", 6, {}, authority);

    expect(published.map(({ subject }) => subject)).to.deep.equal([SUBJECT_RESULT, SUBJECT_RESULT]);
    const trust = {
      audience: SUBJECT_RESULT,
      tenant: "tenant-a",
      sender: "worker-a",
      publicKeys: { "worker-key": keyPair.publicKey },
    };
    const packets = published.map(({ data }) =>
      verifyProductionPacket(data, type, trust) as unknown as VerifiedResultPacket,
    );
    expect({ ...packets[0].jobResult.identity }).to.deep.equal(authority.identity);
    expect({
      ...packets[0].jobResult.dispatch,
      attempt: Number(packets[0].jobResult.dispatch.attempt),
    }).to.deep.equal(authority.dispatch);
    expect({ ...packets[0].identity }).to.deep.equal(authority.identity);
    expect(packets[0].authToken).to.equal("session-a");
    expect(packets[0].signatureMetadata.messageId).to.have.length(16);
    expect(Buffer.from(packets[0].signatureMetadata.messageId).equals(
      Buffer.from(packets[1].signatureMetadata.messageId),
    )).to.equal(false);
  });

  it("rejects an automatic result for another job", async () => {
    const type = await busPacketType();
    const agent = new Agent({
      senderId: "worker-a",
      store: new InMemoryBlobStore(),
      productionTrust: {
        audience: "job.worker.pool-a",
        tenant: "tenant-a",
        sender: "scheduler-a",
        publicKeys: { "scheduler-key": keyPair.publicKey },
      },
      replayStore: new RecordingReplayStore(),
    });
    const runtime = productionRuntime(agent);
    runtime.busPacketType = type;
    runtime.jobResultType = type.root.lookupType("cordum.agent.v1.JobResult");
    runtime.nc = { publish() {} };
    runtime.trust = {
      outboundSessionToken: () => "session-a",
      settings: { config: { proofKeyId: "worker-key", proofPrivateKey: keyPair.privateKey } },
    };
    const request = {
      jobId: "job-a",
      identity: { tenantId: "tenant-a", principalId: "principal-a", actorId: "actor-a" },
      dispatch: { dispatchId: "dispatch-a", attempt: 3, assignedWorkerId: "worker-a" },
    };

    let error: unknown;
    try {
      await runtime.publishResult(
        { packet: { traceId: "trace-a" }, log: console },
        { ...request, jobId: "job-evil" }, "", 0, {}, freezeProductionAuthority(request),
      );
    } catch (caught) {
      error = caught;
    }
    expect(String(error)).to.contain("job id");
  });

  it("requires frozen authority for production results", async () => {
    const type = await busPacketType();
    const agent = new Agent({
      senderId: "worker-a",
      store: new InMemoryBlobStore(),
      productionTrust: {
        audience: "job.worker.pool-a",
        tenant: "tenant-a",
        sender: "scheduler-a",
        publicKeys: { "scheduler-key": keyPair.publicKey },
      },
      replayStore: new RecordingReplayStore(),
    });
    const runtime = productionRuntime(agent);
    runtime.busPacketType = type;
    runtime.jobResultType = type.root.lookupType("cordum.agent.v1.JobResult");
    runtime.nc = { publish() {} };

    let error: unknown;
    try {
      await runtime.publishResult(
        { packet: { traceId: "trace-a" }, log: console },
        { jobId: "job-a" },
        "",
        0,
      );
    } catch (caught) {
      error = caught;
    }
    expect(String(error)).to.contain("requires admitted");
  });

  it("freezes authority before managed handler mutation", async () => {
    const type = await busPacketType();
    const store = new InMemoryBlobStore();
    const published: Array<{ subject: string; data: Buffer }> = [];
    const agent = new Agent({
      senderId: "worker-a",
      store,
      productionTrust: {
        audience: "worker-pool-a",
        tenant: "tenant-a",
        sender: "scheduler-1",
        publicKeys: { k1: keyPair.publicKey },
      },
      replayStore: new RecordingReplayStore(),
    });
    const runtime = productionRuntime(agent);
    runtime.busPacketType = type;
    runtime.jobResultType = type.root.lookupType("cordum.agent.v1.JobResult");
    runtime.nc = {
      publish(subject: string, data: Uint8Array) {
        published.push({ subject, data: Buffer.from(data) });
      },
    };
    runtime.trust = {
      productionSessionActive: true,
      outboundSessionToken: () => "session-a",
      settings: {
        config: {
          proofKeyId: "worker-key",
          proofPrivateKey: keyPair.privateKey,
          tenantId: "tenant-a",
          expectedSchedulerId: "scheduler-1",
        },
      },
    };
    await store.set("ctx:production-echo", Buffer.from("{}"));

    const raw = await signedRawPacket({
      actorId: "actor-a",
      contextPtr: "redis://ctx:production-echo",
      dispatchId: "dispatch-a",
      dispatchAttempt: 3,
      assignedWorkerId: "worker-a",
    });
    await runtime.onMessage(
      { data: raw, subject: "worker-pool-a" },
      {
        topic: "job.test",
        retries: 0,
        async handler(context) {
          if (context.job.identity) context.job.identity.actorId = "actor-evil";
          if (context.job.dispatch) context.job.dispatch.attempt = 99;
          return { ok: true };
        },
      },
    );

    expect(published).to.have.length(1);
    const packet = verifyProductionPacket(published[0].data, type, {
      audience: SUBJECT_RESULT,
      tenant: "tenant-a",
      sender: "worker-a",
      publicKeys: { "worker-key": keyPair.publicKey },
    }) as unknown as VerifiedResultPacket;
    expect(packet.jobResult.identity.actorId).to.equal("actor-a");
    expect(packet.jobResult.dispatch).to.include({
      dispatchId: "dispatch-a",
      assignedWorkerId: "worker-a",
    });
    expect(Number(packet.jobResult.dispatch.attempt)).to.equal(3);
  });
});
