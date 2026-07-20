import type { Type } from "protobufjs";

/** Locally configured authority for CAP-PRODUCTION verification. */
export interface ProductionTrustStore {
  audience: string;
  publicKeys: Record<string, string>;
  tenant: string;
  sender: string;
  maxLifetimeMs?: number;
  clockSkewMs?: number;
  nowMs?: () => number;
}

export interface ProductionTimestampView {
  seconds?: unknown;
  nanos?: unknown;
}

export interface ProductionSignatureMetadataView {
  profileVersion: string;
  algorithm: string;
  messageId: Uint8Array;
  audience: string;
  keyId: string;
  expiresAt: ProductionTimestampView;
}

export interface ProductionIdentityView {
  tenantId?: string;
  principalId?: string;
  actorId?: string;
  delegationId?: string;
}

export interface ProductionJobRequestView {
  tenantId?: string;
  principalId?: string;
  meta?: { tenantId?: string; actorId?: string };
  env?: Record<string, string>;
  identity?: ProductionIdentityView;
  [key: string]: unknown;
}

export interface DecodedProductionPacket {
  signatureMetadata: ProductionSignatureMetadataView;
  identity: ProductionIdentityView;
  senderId: string;
  jobRequest?: ProductionJobRequestView;
  [key: string]: unknown;
}

export interface ProductionPacketType {
  decode(buf: Uint8Array): unknown;
  fieldsById: Type["fieldsById"];
}

export function isProductionRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function validateProductionTrustAuthority(trust: ProductionTrustStore): void {
  const values: unknown[] = [trust.audience, trust.tenant, trust.sender];
  if (values.some((value) =>
    typeof value !== "string" || !value.trim() || value !== value.trim())) {
    throw new Error("production trust authority required");
  }
}
