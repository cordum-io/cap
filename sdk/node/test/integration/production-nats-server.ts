import { execFileSync } from "node:child_process";

import { isNatsReady, reservePort, waitForCondition } from "./nats-server";

const DOCKER_IMAGE = "nats:2.12.6-alpine";

export interface ProductionTestServer {
  readonly url: string;
  start(): Promise<void>;
  stop(strict?: boolean): Promise<void>;
}

interface ServerConstructor {
  new(port: number): ProductionTestServer;
}

class DockerNatsServer implements ProductionTestServer {
  private readonly name: string;

  constructor(private readonly port: number) {
    this.name = `cap-node-production-${process.pid}-${port}`;
  }

  get url(): string {
    return `nats://127.0.0.1:${this.port}`;
  }

  async start(): Promise<void> {
    ensureImage();
    execFileSync("docker", [
      "run", "--detach", "--rm", "--name", this.name,
      "--publish", `127.0.0.1:${this.port}:4222`, DOCKER_IMAGE,
      "-a", "0.0.0.0", "-p", "4222",
    ], { encoding: "utf8", timeout: 20_000, windowsHide: true });
    await waitForCondition(
      () => isNatsReady(this.port),
      8_000,
      `Docker NATS did not listen on ${this.url}`,
    );
  }

  async stop(strict = true): Promise<void> {
    try {
      execFileSync("docker", ["rm", "--force", this.name], {
        encoding: "utf8", timeout: 10_000, windowsHide: true,
      });
      await waitForCondition(
        async () => !(await isNatsReady(this.port)),
        5_000,
        `Docker NATS endpoint ${this.url} remained open`,
      );
    } catch (error) {
      if (strict) throw error;
    }
  }
}

function ensureImage(): void {
  try {
    execFileSync("docker", ["image", "inspect", DOCKER_IMAGE], {
      stdio: "ignore", timeout: 5_000, windowsHide: true,
    });
  } catch {
    execFileSync("docker", ["pull", DOCKER_IMAGE], {
      stdio: "inherit", timeout: 120_000, windowsHide: true,
    });
  }
}

export async function productionNatsServer(
  binaryServer: ServerConstructor,
): Promise<ProductionTestServer> {
  const port = await reservePort();
  return process.env.CAP_NATS_SERVER_BIN
    ? new binaryServer(port)
    : new DockerNatsServer(port);
}
