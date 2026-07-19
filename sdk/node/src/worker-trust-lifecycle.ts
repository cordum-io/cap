import type {
  WorkerHandshakePurpose,
  WorkerHandshakeSession,
} from "./worker-trust-contract";
import type { Logger } from "./logger";
import type { RuntimeTrustSettings } from "./worker-trust-runtime-config";

const PURPOSE_ISSUE: WorkerHandshakePurpose = 1;
const PURPOSE_RENEW: WorkerHandshakePurpose = 2;

export interface WorkerTrustExchange {
  exchange(
    purpose: WorkerHandshakePurpose,
    currentToken: string
  ): Promise<WorkerHandshakeSession>;
}

export class WorkerTrustRuntimeError extends Error {
  readonly cause?: Error;

  constructor(message: string, cause?: Error) {
    super(message);
    this.name = "WorkerTrustRuntimeError";
    this.cause = cause;
  }
}

export class WorkerTrustOperationalError extends Error {
  readonly cause: Error;

  constructor(cause: Error) {
    super("worker trust transport is unavailable");
    this.name = "WorkerTrustOperationalError";
    this.cause = cause;
  }
}

function retryDelayMs(attempt: number): number {
  return Math.min(1000, 25 * (2 ** attempt));
}

function renewalDelayMs(session: WorkerHandshakeSession, now: Date, minimum: number): number {
  const lifetime = Math.max(0, session.expiresAt.getTime() - now.getTime());
  const target = Math.min(lifetime / 2, Math.max(0, lifetime - 60_000));
  return Math.min(Math.max(minimum, target), Math.max(1, lifetime * 0.9));
}

export class WorkerTrustLifecycle {
  private current?: WorkerHandshakeSession;
  private closed = false;
  private queue: Promise<void> = Promise.resolve();
  private renewTask?: Promise<void>;
  private readonly wakeSleepers = new Set<() => void>();
  private readonly closeSignal: Promise<never>;
  private signalClose!: (error: Error) => void;

  constructor(
    private readonly client: WorkerTrustExchange,
    private readonly settings: RuntimeTrustSettings,
    private readonly logger: Logger = console,
    private readonly clock: () => Date = () => new Date()
  ) {
    this.closeSignal = new Promise<never>((_, reject) => { this.signalClose = reject; });
    void this.closeSignal.catch(() => undefined);
  }

  get sessionToken(): string | undefined {
    return this.activeSession()?.token;
  }

  get renewalRunning(): boolean {
    return this.renewTask !== undefined;
  }

  async authenticate(): Promise<boolean> {
    if (this.settings.mode === "off") return false;
    return this.runSerialized(PURPOSE_ISSUE, "");
  }

  async renew(): Promise<boolean> {
    const token = this.sessionToken;
    if (!token) return this.handleFailure(new Error("renew requires a live session"), true);
    return this.runSerialized(PURPOSE_RENEW, token);
  }

  async reauthenticate(): Promise<boolean> {
    this.current = undefined;
    return this.authenticate();
  }

  startRenewal(onEnforceFailure: (error: Error) => Promise<void>): void {
    if (this.closed || this.settings.mode === "off" || !this.sessionToken || this.renewTask) return;
    this.renewTask = this.renewLoop(onEnforceFailure).finally(() => {
      this.renewTask = undefined;
    });
  }

  async close(): Promise<void> {
    if (this.closed) return;
    this.closed = true;
    this.current = undefined;
    this.signalClose(new WorkerTrustRuntimeError("worker trust lifecycle is closed"));
    for (const wake of [...this.wakeSleepers]) wake();
    await this.renewTask;
  }

  private async runSerialized(
    purpose: WorkerHandshakePurpose,
    currentToken: string
  ): Promise<boolean> {
    if (this.closed) throw new WorkerTrustRuntimeError("worker trust lifecycle is closed");
    const work = this.queue.then(() => this.exchangeWithRetries(purpose, currentToken));
    this.queue = work.then(() => undefined, () => undefined);
    return work;
  }

  private async exchangeWithRetries(
    purpose: WorkerHandshakePurpose,
    currentToken: string
  ): Promise<boolean> {
    if (purpose === PURPOSE_RENEW && this.sessionToken !== currentToken) return true;
    let failure: Error = new Error("worker trust exchange failed");
    for (let attempt = 0; attempt < this.settings.retries; attempt += 1) {
      try {
        const session = await Promise.race([
          this.client.exchange(purpose, currentToken),
          this.closeSignal,
        ]);
        if (this.closed) throw new WorkerTrustRuntimeError("worker trust lifecycle is closed");
        this.current = session;
        return true;
      } catch (error) {
        failure = error instanceof Error ? error : new Error(String(error));
        if (this.closed) throw failure;
        if (!(failure instanceof WorkerTrustOperationalError)) break;
        if (attempt + 1 < this.settings.retries) await this.sleep(retryDelayMs(attempt));
      }
    }
    return this.handleFailure(failure, purpose === PURPOSE_RENEW);
  }

  private handleFailure(error: Error, renewal: boolean): false {
    const operational = error instanceof WorkerTrustOperationalError;
    if (renewal && (this.settings.mode === "enforce" || !operational)) {
      this.current = undefined;
    }
    else this.activeSession();
    if (this.settings.mode === "warn" && operational) {
      this.logger.warn("worker trust transport unavailable; continuing without new session", {
        errorType: error.name,
      });
      return false;
    }
    throw new WorkerTrustRuntimeError("authenticated worker trust failed", error);
  }

  private async renewLoop(onEnforceFailure: (error: Error) => Promise<void>): Promise<void> {
    while (!this.closed) {
      const session = this.activeSession();
      if (!session) return;
      await this.sleep(renewalDelayMs(session, this.clock(), this.settings.renewMinIntervalMs));
      if (this.closed) return;
      try {
        if (await this.renew()) continue;
      } catch (error) {
        if (this.closed) return;
        await onEnforceFailure(error instanceof Error ? error : new Error(String(error)));
        return;
      }
      if (!this.activeSession()) return;
      await this.sleep(this.settings.renewMinIntervalMs);
    }
  }

  private activeSession(): WorkerHandshakeSession | undefined {
    if (this.current && this.current.expiresAt.getTime() > this.clock().getTime()) return this.current;
    this.current = undefined;
    return undefined;
  }

  private sleep(milliseconds: number): Promise<void> {
    return new Promise((resolve) => {
      let pending = true;
      const done = () => {
        if (!pending) return;
        pending = false;
        clearTimeout(handle);
        this.wakeSleepers.delete(done);
        resolve();
      };
      const handle = setTimeout(done, milliseconds);
      this.wakeSleepers.add(done);
    });
  }
}
