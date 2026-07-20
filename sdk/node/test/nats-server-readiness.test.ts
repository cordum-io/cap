import assert from "node:assert/strict";
import net from "node:net";
import { isNatsReady, reservePort } from "./integration/nats-server";

async function listenWithoutNatsProtocol(port: number): Promise<net.Server> {
  const server = net.createServer(() => undefined);
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(port, "127.0.0.1", resolve);
  });
  return server;
}

async function closeServer(server: net.Server): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
  });
}

describe("NATS server readiness", () => {
  it("does not treat a TCP-only listener as a ready NATS server", async () => {
    const server = await listenWithoutNatsProtocol(await reservePort());
    const address = server.address();
    assert.ok(address && typeof address !== "string");
    try {
      assert.equal(await isNatsReady(address.port), false);
    } finally {
      await closeServer(server);
    }
  });
});
