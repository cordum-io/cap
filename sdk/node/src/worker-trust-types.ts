import { ProtoTimestamp } from "./worker-trust-contract";

export interface WorkerHandshakeChallengeRequest {
  requestId: string;
  traceId: string;
  workerId: string;
  proofKeyId: string;
  proofAlgorithm: number;
  audience: string;
  purpose: number;
  clientNonce: Uint8Array;
  protocolVersion: number;
  sdkVersion: string;
}

export interface WorkerHandshakeChallenge {
  requestId: string;
  challengeId: string;
  traceId: string;
  workerId: string;
  agentId: string;
  tenantId: string;
  proofKeyId: string;
  proofAlgorithm: number;
  serverKeyId: string;
  audience: string;
  purpose: number;
  clientNonce: Uint8Array;
  serverNonce: Uint8Array;
  protocolVersion: number;
  sdkVersion: string;
  issuedAt: ProtoTimestamp;
  expiresAt: ProtoTimestamp;
}

export interface WorkerCapabilityHandshake {
  componentId: string;
  role: number;
  supportedVersions: number[];
  capabilities: Record<string, boolean>;
  sdkVersion: string;
  readyTopics: string[];
  agentName?: string;
}

export interface WorkerHandshakeAuthenticate {
  challenge: WorkerHandshakeChallenge;
  capabilityHandshake: WorkerCapabilityHandshake;
}

export interface WorkerHandshakeResult {
  challenge: WorkerHandshakeChallenge;
  accepted: boolean;
  rejectionReason: number;
  tokenExpiresAt?: ProtoTimestamp;
  issuedAt: ProtoTimestamp;
}

export interface WorkerTrustEnvelope {
  traceId: string;
  senderId: string;
  createdAt: ProtoTimestamp;
  protocolVersion: number;
  signature: Uint8Array;
  authToken?: string;
}

export interface ChallengeRequestTrustPacket extends WorkerTrustEnvelope {
  workerHandshakeChallengeRequest: WorkerHandshakeChallengeRequest;
}

export interface ChallengeTrustPacket extends WorkerTrustEnvelope {
  workerHandshakeChallenge: WorkerHandshakeChallenge;
}

export interface AuthenticateTrustPacket extends WorkerTrustEnvelope {
  workerHandshakeAuthenticate: WorkerHandshakeAuthenticate;
}

export interface ResultTrustPacket extends WorkerTrustEnvelope {
  workerHandshakeResult: WorkerHandshakeResult;
}

export type WorkerTrustPacket =
  | ChallengeRequestTrustPacket
  | ChallengeTrustPacket
  | AuthenticateTrustPacket
  | ResultTrustPacket;

export type WorkerTrustPhase =
  | "challenge_request"
  | "challenge"
  | "authenticate"
  | "result";
