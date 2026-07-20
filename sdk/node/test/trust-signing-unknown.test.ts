import * as assert from "node:assert";
import * as fs from "node:fs";
import * as path from "node:path";

import { loadRoot } from "../src/protos";
import {
  ChallengeTrustPacket,
  verifyWorkerTrustPacket,
  workerTrustDigest,
} from "../src/trust-signing";

function fixtureDir(): string {
  return path.resolve(__dirname, "..", "..", "..", "..", "spec", "conformance", "fixtures", "handshake");
}

describe("worker trust signing input validation", () => {
  it("rejects nested unknown object fields before digest or verification", async () => {
    const root = await loadRoot();
    const type = root.lookupType("cordum.agent.v1.BusPacket");
    const manifest = JSON.parse(
      fs.readFileSync(path.join(fixtureDir(), "manifest.json"), "utf8")
    );
    const packet = type.decode(
      fs.readFileSync(path.join(fixtureDir(), manifest.positive.challenge.packet))
    ) as unknown as ChallengeTrustPacket;
    (packet.workerHandshakeChallenge as unknown as Record<string, unknown>).attacker = 1;
    const schedulerKey = fs.readFileSync(path.join(fixtureDir(), "scheduler_public.pem"), "utf8");

    await assert.rejects(() => workerTrustDigest(packet), /unknown field/i);
    await assert.rejects(
      () => verifyWorkerTrustPacket(packet, { "scheduler-key-v1": schedulerKey }),
      /unknown field/i
    );
  });
});
