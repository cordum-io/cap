import type {
  Msg,
  NatsConnection,
  Status,
  Subscription,
  SubscriptionOptions,
} from "nats";

import { FakeTrustRequester, type TrustFixture } from "./worker-trust-runtime-support";

interface PublishedMessage {
  readonly subject: string;
  readonly data: Uint8Array;
}

class StatusQueue implements AsyncIterable<Status>, AsyncIterator<Status> {
  private readonly queued: Status[] = [];
  private readonly waiting: Array<(value: IteratorResult<Status>) => void> = [];
  private ended = false;

  [Symbol.asyncIterator](): AsyncIterator<Status> {
    return this;
  }

  next(): Promise<IteratorResult<Status>> {
    const value = this.queued.shift();
    if (value) return Promise.resolve({ value, done: false });
    if (this.ended) return Promise.resolve({ value: undefined, done: true });
    return new Promise((resolve) => this.waiting.push(resolve));
  }

  push(value: Status): void {
    const waiter = this.waiting.shift();
    if (waiter) waiter({ value, done: false });
    else this.queued.push(value);
  }

  end(): void {
    this.ended = true;
    for (const waiter of this.waiting.splice(0)) waiter({ value: undefined, done: true });
  }
}

class FakeSubscription {
  closed = false;

  constructor(
    readonly subject: string,
    private readonly onDrain: () => void
  ) {}

  async drain(): Promise<void> {
    if (!this.closed) this.onDrain();
    this.closed = true;
  }

  unsubscribe(): void {
    if (!this.closed) this.onDrain();
    this.closed = true;
  }
}

export class RuntimeTrustConnection extends FakeTrustRequester {
  readonly events: string[] = [];
  readonly published: PublishedMessage[] = [];
  readonly subscriptions: FakeSubscription[] = [];
  private readonly callbacks = new Map<string, SubscriptionOptions["callback"]>();
  private readonly statuses = new StatusQueue();
  private closedConnection = false;

  constructor(fixture: TrustFixture, clock: () => Date = () => new Date()) {
    super(fixture, clock);
  }

  override async request(
    subject: string,
    data: Uint8Array,
    options?: { timeout?: number }
  ): Promise<{ data: Uint8Array }> {
    this.events.push(`request:${subject}`);
    return super.request(subject, data, options);
  }

  publish(subject: string, data: Uint8Array = new Uint8Array()): void {
    this.events.push(`publish:${subject}`);
    this.published.push({ subject, data });
  }

  subscribe(subject: string, options: SubscriptionOptions = {}): Subscription {
    this.events.push(`subscribe:${subject}`);
    if (options.callback) this.callbacks.set(subject, options.callback);
    const subscription = new FakeSubscription(subject, () => {
      this.events.push(`drain:${subject}`);
      this.callbacks.delete(subject);
    });
    this.subscriptions.push(subscription);
    return subscription as unknown as Subscription;
  }

  deliver(subject: string, data: Uint8Array): void {
    const callback = this.callbacks.get(subject);
    if (!callback) throw new Error(`no active subscription for ${subject}`);
    callback(null, { data, subject } as Msg);
  }

  emitStatus(status: Status): void {
    this.statuses.push(status);
  }

  status(): AsyncIterable<Status> {
    return this.statuses;
  }

  async drain(): Promise<void> {
    this.events.push("drain:connection");
    this.closedConnection = true;
    this.statuses.end();
  }

  async close(): Promise<void> {
    this.events.push("close:connection");
    this.closedConnection = true;
    this.statuses.end();
  }

  isClosed(): boolean {
    return this.closedConnection;
  }

  asNatsConnection(): NatsConnection {
    return this as unknown as NatsConnection;
  }
}

export async function waitFor(
  predicate: () => boolean,
  timeoutMs = 1000
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (!predicate()) {
    if (Date.now() >= deadline) throw new Error("condition was not met");
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
}
