import { expect } from "chai";

import type { Logger } from "../src/logger";
import {
  ReplayConflictError,
  ReplayOutcome,
  ReplayStoreUnavailableError,
  type ReplayStore,
} from "../src/production-replay";
import {
  ProductionWireError,
  verifyProductionPacket,
  type ProductionTrustStore,
} from "../src/production-signing";
import { Agent, InMemoryBlobStore } from "../src/runtime";
import {
  busPacketType,
  keyPair,
  signedRawPacket,
} from "./production-admission-support";

interface AdmissionHarness {
  busPacketType: unknown;
  decodeProductionPacket(raw: Uint8Array): Promise<unknown | null>;
}

function admissionHarness(agent: Agent): AdmissionHarness {
  return agent as unknown as AdmissionHarness;
}

function recordingLogger(errors: string[]): Logger {
  return {
    info: () => undefined,
    error: () => undefined,
    warn: (_message, fields) => errors.push(String(fields?.error ?? "")),
  };
}

async function productionAgent(
  replayStore: ReplayStore,
  errors: string[] = [],
  productionTrust: ProductionTrustStore = {
    audience: "worker-pool-a",
    tenant: "tenant-a",
    sender: "scheduler-1",
    publicKeys: { k1: keyPair.publicKey },
  },
): Promise<Agent> {
  const agent = new Agent({
    store: new InMemoryBlobStore(),
    productionTrust,
    replayStore,
    logger: recordingLogger(errors),
  });
  admissionHarness(agent).busPacketType = await busPacketType();
  return agent;
}

describe("CAP-PRODUCTION verifier authority hardening", () => {
  it("rejects inherited and prototype-like key ids", async () => {
    const type = await busPacketType();
    const inheritedKeys = Object.create({ inherited: keyPair.publicKey }) as Record<string, string>;
    const cases: Array<[string, Record<string, string>]> = [
      ["inherited", inheritedKeys],
      ["__proto__", {}],
    ];

    for (const [keyId, publicKeys] of cases) {
      const raw = await signedRawPacket({ keyId });
      expect(() => verifyProductionPacket(raw, type, {
        audience: "worker-pool-a", tenant: "tenant-a", sender: "scheduler-1", publicKeys,
      })).to.throw(ProductionWireError, "unknown key id");
    }
  });

  it("rejects padded trust authorities without normalizing them", async () => {
    const raw = await signedRawPacket();
    const type = await busPacketType();
    const base = {
      audience: "worker-pool-a", tenant: "tenant-a", sender: "scheduler-1",
      publicKeys: { k1: keyPair.publicKey },
    };

    for (const field of ["audience", "tenant", "sender"] as const) {
      const trust = { ...base, [field]: ` ${base[field]}` };
      expect(() => verifyProductionPacket(raw, type, trust))
        .to.throw(ProductionWireError, "production trust authority required");
    }
  });
});

describe("CAP-PRODUCTION runtime hardening", () => {
  it("snapshots caller-owned trust and key mappings", async () => {
    const publicKeys: Record<string, string> = { k1: keyPair.publicKey };
    const trust = {
      audience: "worker-pool-a",
      tenant: "tenant-a",
      sender: "scheduler-1",
      publicKeys,
    };
    const store = { admit: () => ReplayOutcome.First };
    const agent = await productionAgent(store, [], trust);
    trust.audience = "mutated-audience";
    delete publicKeys.k1;

    expect(await admissionHarness(agent).decodeProductionPacket(await signedRawPacket()))
      .to.not.equal(null);
  });

  const secret = "redis://user:password@private-host";
  const failures: Array<[string, Error, string]> = [
    ["conflict", new ReplayConflictError(secret), "replay conflict"],
    ["unavailable", new ReplayStoreUnavailableError(secret), "replay store unavailable"],
    ["unexpected", new Error(secret), "replay store unavailable"],
  ];
  for (const [name, error, category] of failures) {
    it(`normalizes ${name} replay failures`, async () => {
      const errors: string[] = [];
      const store = { admit: () => { throw error; } };
      const agent = await productionAgent(store, errors);

      expect(await admissionHarness(agent).decodeProductionPacket(await signedRawPacket()))
        .to.equal(null);
      expect(errors).to.deep.equal([category]);
      expect(errors.join(" ")).not.to.contain(secret);
    });
  }

  it("normalizes an invalid replay outcome", async () => {
    const errors: string[] = [];
    const store = { admit: () => secret as ReplayOutcome };
    const agent = await productionAgent(store, errors);

    expect(await admissionHarness(agent).decodeProductionPacket(await signedRawPacket()))
      .to.equal(null);
    expect(errors).to.deep.equal(["invalid outcome"]);
    expect(errors.join(" ")).not.to.contain(secret);
  });
});
