import assert from "node:assert";
import { Agent, InMemoryBlobStore } from "../src/runtime";
import { loadRoot } from "../src/protos";

class MockNatsConnection {
  published: Array<{ subject: string; data: Uint8Array }> = [];
  subscriptions: Map<string, (msg: any) => void> = new Map();

  async publish(subject: string, data: Uint8Array): Promise<void> {
    this.published.push({ subject, data });
  }

  subscribe(subject: string, _opts?: any, cb?: (msg: any) => void): any {
    if (cb) {
      this.subscriptions.set(subject, cb);
    }
    return {
      drain: async () => {},
    };
  }

  async drain(): Promise<void> {
    return;
  }
}

describe("handshake helpers", () => {
  it("publishes handshake during Agent.start", async () => {
    const store = new InMemoryBlobStore();
    const mock = new MockNatsConnection();
    const agent = new Agent({
      store,
      connectFn: async () => mock as any,
      senderId: "worker-handshake",
    });

    agent.job("job.handshake", async () => {
      return {};
    });
    agent.job("job.handshake.secondary", async () => {
      return {};
    });

    await agent.start();

    assert.ok(mock.published.length > 0);
    assert.strictEqual(mock.published[0].subject, "sys.handshake");

    const root = await loadRoot();
    const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");
    const packet = BusPacket.decode(mock.published[0].data) as any;

    assert.strictEqual(packet.senderId, "worker-handshake");
    assert.strictEqual(packet.handshake.componentId, "worker-handshake");
    assert.strictEqual(packet.handshake.role, 3);
    assert.deepStrictEqual(packet.handshake.supportedVersions, [1]);
    assert.strictEqual(packet.handshake.capabilities["job.handshake"], true);
    assert.strictEqual(packet.handshake.capabilities["job.handshake.secondary"], true);
    assert.deepStrictEqual(packet.handshake.readyTopics, ["job.handshake", "job.handshake.secondary"]);

    await agent.close();
  });
});
