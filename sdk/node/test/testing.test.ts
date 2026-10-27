import assert from "node:assert";
import { testHandler, createTestAgent, MockNatsConnection } from "../src/testing";
import { z } from "zod";
import type { Context, JobSchema } from "../src/runtime";

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

  it("accepts any schema with parse(), not only zod", async () => {
    const Input: JobSchema<{ n: number }> = {
      parse(data: unknown) {
        const n = (data as { n?: unknown } | null)?.n;
        if (typeof n !== "number") throw new Error("n must be a number");
        return { n };
      },
    };
    const parsedOutputs: unknown[] = [];
    const Output: JobSchema<{ doubled: number }> = {
      parse(data: unknown) {
        parsedOutputs.push(data);
        return data as { doubled: number };
      },
    };
    const handler = async (_ctx: Context, data: { n: number }) => ({ doubled: data.n * 2 });

    const ok = await testHandler(handler, { n: 2 }, { inputSchema: Input, outputSchema: Output });
    assert.strictEqual(ok.status, 5); // JOB_STATUS_SUCCEEDED
    assert.deepStrictEqual(parsedOutputs, [{ doubled: 4 }]);

    const bad = await testHandler(handler, { n: "2" } as unknown as { n: number }, { inputSchema: Input });
    assert.strictEqual(bad.status, 6); // JOB_STATUS_FAILED
    assert.match(bad.errorMessage, /input validation failed: Error: n must be a number/);

    const { agent } = createTestAgent();
    agent.job("job.typed", Input, async (_ctx, data) => {
      // @ts-expect-error data is inferred from Input.parse, so n is a number
      const wrong: string = data.n;
      return { doubled: data.n * 2, wrong };
    });
  });

  it("createTestAgent provides pre-wired agent", async () => {
    const { agent, bus, store } = createTestAgent();

    assert.ok(agent);
    assert.ok(bus instanceof MockNatsConnection);
    assert.ok(store);
  });
});
