import assert from "node:assert";
import { testHandler, createTestAgent, MockNatsConnection } from "../src/testing";
import { z } from "zod";
import type { Context } from "../src/runtime";

describe("CAP testing utilities", () => {
  it("runs echo handler without NATS or Redis", async () => {
    const handler = async (_ctx: Context, data: { prompt: string }) => {
      return { echo: data.prompt };
    };

    const result = await testHandler(handler, { prompt: "hello" }, {
      jobId: "test-1",
      topic: "job.echo",
    });

    assert.strictEqual(result.status, 5); // JOB_STATUS_SUCCEEDED
    assert.strictEqual(result.jobId, "test-1");
  });

  it("runs handler with Zod validation", async () => {
    const Input = z.object({ prompt: z.string() });
    const Output = z.object({ echo: z.string() });

    const handler = async (_ctx: Context, data: z.infer<typeof Input>) => {
      return { echo: data.prompt.toUpperCase() };
    };

    const result = await testHandler(handler, { prompt: "hello" }, {
      inputSchema: Input,
      outputSchema: Output,
    });

    assert.strictEqual(result.status, 5); // JOB_STATUS_SUCCEEDED
  });

  it("createTestAgent provides pre-wired agent", async () => {
    const { agent, bus, store } = createTestAgent();

    assert.ok(agent);
    assert.ok(bus instanceof MockNatsConnection);
    assert.ok(store);
  });
});
