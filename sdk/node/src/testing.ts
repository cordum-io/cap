/**
 * Testing utilities for CAP Node SDK.
 * Allows testing handlers without running NATS or Redis.
 */
import type { Msg, NatsConnection, Subscription, SubscriptionOptions } from "nats";
import { z } from "zod";
import { Agent, InMemoryBlobStore, AgentOptions, Context } from "./runtime";
import { loadRoot, DEFAULT_PROTOCOL_VERSION } from "./protos";
import { encodeDeterministic } from "./codec";

/** A message published on the mock bus. */
export interface PublishedMsg {
  subject: string;
  data: Uint8Array;
}

/**
 * MockNatsConnection simulates a NATS connection for testing.
 * Tracks published messages and subscription callbacks.
 */
export class MockNatsConnection {
  published: PublishedMsg[] = [];
  subscriptions: Map<
    string,
    NonNullable<SubscriptionOptions["callback"]>
  > = new Map();

  async publish(subject: string, data: Uint8Array): Promise<void> {
    this.published.push({ subject, data });
  }

  subscribe(subject: string, opts: SubscriptionOptions = {}): Subscription {
    if (opts.callback) {
      this.subscriptions.set(subject, opts.callback);
    }
    return {
      drain: async () => {},
      unsubscribe: () => {},
    } as unknown as Subscription;
  }

  deliver(subject: string, data: Uint8Array): void {
    const callback = this.subscriptions.get(subject);
    if (!callback) {
      throw new Error(`no callback subscription found for subject ${subject}`);
    }
    callback(null, { subject, data } as unknown as Msg);
  }

  async drain(): Promise<void> {
    return;
  }
}

/**
 * Creates an Agent pre-configured with MockNatsConnection + InMemoryBlobStore.
 * No NATS or Redis required.
 */
export function createTestAgent(
  options: Partial<AgentOptions> = {}
): { agent: Agent; bus: MockNatsConnection; store: InMemoryBlobStore } {
  const bus = new MockNatsConnection();
  const store = new InMemoryBlobStore();
  const agent = new Agent({
    store,
    connectFn: async () => bus as unknown as NatsConnection,
    senderId: options.senderId ?? "test-worker",
    retries: options.retries ?? 0,
    ...options,
  });
  return { agent, bus, store };
}

async function waitForPublish(bus: MockNatsConnection, initialCount: number, timeoutMs = 1000): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (bus.published.length > initialCount) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  throw new Error("timeout waiting for handler to publish result");
}

/**
 * Runs a single handler invocation and returns the decoded JobResult.
 * No NATS or Redis required.
 *
 * @param handler - The handler function to test
 * @param input - The input data to pass to the handler
 * @param options - Optional: inputSchema, outputSchema, senderId, retries
 * @returns The decoded JobResult from the handler
 */
export async function testHandler<TIn, TOut>(
  handler: (ctx: Context, data: TIn) => Promise<TOut> | TOut,
  input: TIn,
  options: {
    inputSchema?: z.ZodType<TIn>;
    outputSchema?: z.ZodType<TOut>;
    senderId?: string;
    retries?: number;
    jobId?: string;
    topic?: string;
  } = {}
): Promise<{ status: number; jobId: string; resultPtr: string; errorMessage: string; workerId: string }> {
  const { agent, bus, store } = createTestAgent({
    senderId: options.senderId,
    retries: options.retries,
  });

  const topic = options.topic ?? "job.test";
  const jobId = options.jobId ?? "test-job-1";

  if (options.inputSchema) {
    agent.job<TIn, TOut>(topic, options.inputSchema, handler, {
      outputSchema: options.outputSchema,
    });
  } else {
    agent.job<TIn, TOut>(topic, handler, {
      outputSchema: options.outputSchema,
    });
  }

  await agent.start();

  const ctxKey = `ctx:${jobId}`;
  await store.set(ctxKey, Buffer.from(JSON.stringify(input), "utf-8"));

  const root = await loadRoot();
  const BusPacket = root.lookupType("cordum.agent.v1.BusPacket");

  const packet = BusPacket.fromObject({
    traceId: "test-trace",
    senderId: "test-client",
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    createdAt: { seconds: Math.floor(Date.now() / 1000), nanos: 0 },
    jobRequest: { jobId, topic, contextPtr: `redis://${ctxKey}` },
  });
  const payload = encodeDeterministic(BusPacket, packet);

  const initialCount = bus.published.length;
  bus.deliver(topic, payload);

  await waitForPublish(bus, initialCount);

  const resultPacket = BusPacket.decode(
    bus.published[bus.published.length - 1].data
  ) as unknown as { jobResult?: JobResultView };
  const jr = resultPacket.jobResult;
  if (!jr) {
    throw new Error("worker did not publish a JobResult");
  }

  await agent.close();

  return {
    status: jr.status,
    jobId: jr.jobId,
    resultPtr: jr.resultPtr ?? "",
    errorMessage: jr.errorMessage ?? "",
    workerId: jr.workerId ?? "",
  };
}

interface JobResultView {
  status: number;
  jobId: string;
  resultPtr?: string;
  errorMessage?: string;
  workerId?: string;
}
