import {
  WorkerCapabilityHandshake,
  WorkerHandshakeChallenge,
} from "./worker-trust-types";

export function cloneChallenge(
  value: WorkerHandshakeChallenge
): WorkerHandshakeChallenge {
  return {
    ...value,
    clientNonce: new Uint8Array(value.clientNonce),
    serverNonce: new Uint8Array(value.serverNonce),
    issuedAt: { ...value.issuedAt },
    expiresAt: { ...value.expiresAt },
  };
}

export function cloneCapability(
  value: WorkerCapabilityHandshake
): WorkerCapabilityHandshake {
  return {
    ...value,
    supportedVersions: [...(value.supportedVersions ?? [])],
    capabilities: { ...(value.capabilities ?? {}) },
    readyTopics: [...(value.readyTopics ?? [])],
  };
}

export function sameBytes(left: Uint8Array, right: Uint8Array): boolean {
  return Buffer.from(left).equals(Buffer.from(right));
}
