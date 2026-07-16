import { ChildProcess, execFileSync, spawn } from "node:child_process";
import net from "node:net";
import { setTimeout as delay } from "node:timers/promises";

const NATS_IMAGE = "nats:2.10.25-alpine";
const NATS_VERSION = "2.10.25";
const START_TIMEOUT_MS = 8_000;

interface CommandSpec {
  command: string;
  args: string[];
}

export class NatsServer {
  private child?: ChildProcess;
  private output = "";
  private readonly binary =
    process.env.CAP_NATS_SERVER_BIN ?? process.env.NATS_SERVER_BIN;
  private readonly containerName = `cap-node-e2e-${process.pid}-${Date.now()}`;

  constructor(readonly port: number) {}

  get url(): string {
    return `nats://127.0.0.1:${this.port}`;
  }

  async start(): Promise<void> {
    if (this.child?.exitCode === null) {
      throw new Error("NATS server is already running");
    }
    const spec = this.command();
    this.output = "";
    let spawnError: Error | undefined;
    const child = spawn(spec.command, spec.args, {
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
    });
    child.once("error", (error: Error) => {
      spawnError = error;
    });
    child.stdout?.on("data", (chunk: Buffer) => this.appendOutput(chunk));
    child.stderr?.on("data", (chunk: Buffer) => this.appendOutput(chunk));
    this.child = child;
    await waitForCondition(async () => {
      if (spawnError) throw spawnError;
      if (child.exitCode !== null) {
        throw new Error(`NATS exited ${child.exitCode}: ${this.output}`);
      }
      return canConnect(this.port);
    }, START_TIMEOUT_MS, `NATS did not listen on ${this.url}: ${this.output}`);
  }

  async stop(strict = true): Promise<void> {
    const child = this.child;
    if (!child) return;
    try {
      if (child.exitCode === null && this.binary) {
        child.kill();
      } else if (child.exitCode === null) {
        execFileSync("docker", ["stop", "-t", "1", this.containerName], {
          stdio: "pipe",
          timeout: 10_000,
          windowsHide: true,
        });
      }
      await waitForExit(child);
      await waitForCondition(
        async () => !(await canConnect(this.port)),
        5_000,
        `NATS endpoint ${this.url} remained open`,
      );
    } catch (error) {
      if (strict) throw error;
    } finally {
      this.child = undefined;
    }
  }

  private command(): CommandSpec {
    if (this.binary) {
      const version = execFileSync(this.binary, ["-v"], {
        encoding: "utf8",
        timeout: 5_000,
        windowsHide: true,
      });
      if (!version.includes(`v${NATS_VERSION}`)) {
        throw new Error(`expected nats-server v${NATS_VERSION}, got ${version.trim()}`);
      }
      return {
        command: this.binary,
        args: ["-a", "127.0.0.1", "-p", String(this.port)],
      };
    }
    return {
      command: "docker",
      args: [
        "run", "--rm", "--name", this.containerName,
        "-p", `127.0.0.1:${this.port}:4222`, NATS_IMAGE,
      ],
    };
  }

  private appendOutput(chunk: Buffer): void {
    this.output = `${this.output}${chunk.toString("utf8")}`.slice(-8_192);
  }
}

export async function reservePort(): Promise<number> {
  return new Promise<number>((resolve, reject) => {
    const server = net.createServer();
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      if (!address || typeof address === "string") {
        server.close();
        reject(new Error("failed to reserve a TCP port"));
        return;
      }
      server.close((error) => error ? reject(error) : resolve(address.port));
    });
  });
}

async function canConnect(port: number): Promise<boolean> {
  return new Promise<boolean>((resolve) => {
    const socket = net.createConnection({ host: "127.0.0.1", port });
    let finished = false;
    const finish = (connected: boolean): void => {
      if (finished) return;
      finished = true;
      socket.destroy();
      resolve(connected);
    };
    socket.setTimeout(250, () => finish(false));
    socket.once("connect", () => finish(true));
    socket.once("error", () => finish(false));
  });
}

export async function waitForCondition(
  check: () => boolean | Promise<boolean>,
  timeoutMs: number,
  message: string,
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await check()) return;
    await delay(50);
  }
  throw new Error(message);
}

async function waitForExit(child: ChildProcess): Promise<void> {
  if (child.exitCode !== null) return;
  await withTimeout(
    new Promise<void>((resolve) => child.once("exit", () => resolve())),
    5_000,
    "NATS server did not exit",
  );
}

export async function withTimeout<T>(
  promise: Promise<T>,
  timeoutMs: number,
  message: string,
): Promise<T> {
  let timer: NodeJS.Timeout | undefined;
  const timeout = new Promise<never>((_resolve, reject) => {
    timer = setTimeout(() => reject(new Error(message)), timeoutMs);
  });
  try {
    return await Promise.race([promise, timeout]);
  } finally {
    if (timer) clearTimeout(timer);
  }
}
