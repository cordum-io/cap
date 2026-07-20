// package: cordum.agent.v1
// file: cordum/agent/v1/handshake.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class Handshake extends jspb.Message {
  getComponentId(): string;
  setComponentId(value: string): void;

  getRole(): ComponentRoleMap[keyof ComponentRoleMap];
  setRole(value: ComponentRoleMap[keyof ComponentRoleMap]): void;

  clearSupportedVersionsList(): void;
  getSupportedVersionsList(): Array<number>;
  setSupportedVersionsList(value: Array<number>): void;
  addSupportedVersions(value: number, index?: number): number;

  getCapabilitiesMap(): jspb.Map<string, boolean>;
  clearCapabilitiesMap(): void;
  getSdkVersion(): string;
  setSdkVersion(value: string): void;

  clearReadyTopicsList(): void;
  getReadyTopicsList(): Array<string>;
  setReadyTopicsList(value: Array<string>): void;
  addReadyTopics(value: string, index?: number): string;

  getAgentName(): string;
  setAgentName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Handshake.AsObject;
  static toObject(includeInstance: boolean, msg: Handshake): Handshake.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Handshake, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Handshake;
  static deserializeBinaryFromReader(message: Handshake, reader: jspb.BinaryReader): Handshake;
}

export namespace Handshake {
  export type AsObject = {
    componentId: string,
    role: ComponentRoleMap[keyof ComponentRoleMap],
    supportedVersionsList: Array<number>,
    capabilitiesMap: Array<[string, boolean]>,
    sdkVersion: string,
    readyTopicsList: Array<string>,
    agentName: string,
  }
}

export class WorkerHandshakeChallengeRequest extends jspb.Message {
  getRequestId(): string;
  setRequestId(value: string): void;

  getTraceId(): string;
  setTraceId(value: string): void;

  getWorkerId(): string;
  setWorkerId(value: string): void;

  getProofKeyId(): string;
  setProofKeyId(value: string): void;

  getProofAlgorithm(): WorkerHandshakeProofAlgorithmMap[keyof WorkerHandshakeProofAlgorithmMap];
  setProofAlgorithm(value: WorkerHandshakeProofAlgorithmMap[keyof WorkerHandshakeProofAlgorithmMap]): void;

  getAudience(): string;
  setAudience(value: string): void;

  getPurpose(): WorkerHandshakePurposeMap[keyof WorkerHandshakePurposeMap];
  setPurpose(value: WorkerHandshakePurposeMap[keyof WorkerHandshakePurposeMap]): void;

  getClientNonce(): Uint8Array | string;
  getClientNonce_asU8(): Uint8Array;
  getClientNonce_asB64(): string;
  setClientNonce(value: Uint8Array | string): void;

  getProtocolVersion(): number;
  setProtocolVersion(value: number): void;

  getSdkVersion(): string;
  setSdkVersion(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): WorkerHandshakeChallengeRequest.AsObject;
  static toObject(includeInstance: boolean, msg: WorkerHandshakeChallengeRequest): WorkerHandshakeChallengeRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: WorkerHandshakeChallengeRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): WorkerHandshakeChallengeRequest;
  static deserializeBinaryFromReader(message: WorkerHandshakeChallengeRequest, reader: jspb.BinaryReader): WorkerHandshakeChallengeRequest;
}

export namespace WorkerHandshakeChallengeRequest {
  export type AsObject = {
    requestId: string,
    traceId: string,
    workerId: string,
    proofKeyId: string,
    proofAlgorithm: WorkerHandshakeProofAlgorithmMap[keyof WorkerHandshakeProofAlgorithmMap],
    audience: string,
    purpose: WorkerHandshakePurposeMap[keyof WorkerHandshakePurposeMap],
    clientNonce: Uint8Array | string,
    protocolVersion: number,
    sdkVersion: string,
  }
}

export class WorkerHandshakeChallenge extends jspb.Message {
  getRequestId(): string;
  setRequestId(value: string): void;

  getChallengeId(): string;
  setChallengeId(value: string): void;

  getTraceId(): string;
  setTraceId(value: string): void;

  getWorkerId(): string;
  setWorkerId(value: string): void;

  getAgentId(): string;
  setAgentId(value: string): void;

  getTenantId(): string;
  setTenantId(value: string): void;

  getProofKeyId(): string;
  setProofKeyId(value: string): void;

  getProofAlgorithm(): WorkerHandshakeProofAlgorithmMap[keyof WorkerHandshakeProofAlgorithmMap];
  setProofAlgorithm(value: WorkerHandshakeProofAlgorithmMap[keyof WorkerHandshakeProofAlgorithmMap]): void;

  getServerKeyId(): string;
  setServerKeyId(value: string): void;

  getAudience(): string;
  setAudience(value: string): void;

  getPurpose(): WorkerHandshakePurposeMap[keyof WorkerHandshakePurposeMap];
  setPurpose(value: WorkerHandshakePurposeMap[keyof WorkerHandshakePurposeMap]): void;

  getClientNonce(): Uint8Array | string;
  getClientNonce_asU8(): Uint8Array;
  getClientNonce_asB64(): string;
  setClientNonce(value: Uint8Array | string): void;

  getServerNonce(): Uint8Array | string;
  getServerNonce_asU8(): Uint8Array;
  getServerNonce_asB64(): string;
  setServerNonce(value: Uint8Array | string): void;

  getProtocolVersion(): number;
  setProtocolVersion(value: number): void;

  getSdkVersion(): string;
  setSdkVersion(value: string): void;

  hasIssuedAt(): boolean;
  clearIssuedAt(): void;
  getIssuedAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setIssuedAt(value?: google_protobuf_timestamp_pb.Timestamp): void;

  hasExpiresAt(): boolean;
  clearExpiresAt(): void;
  getExpiresAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setExpiresAt(value?: google_protobuf_timestamp_pb.Timestamp): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): WorkerHandshakeChallenge.AsObject;
  static toObject(includeInstance: boolean, msg: WorkerHandshakeChallenge): WorkerHandshakeChallenge.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: WorkerHandshakeChallenge, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): WorkerHandshakeChallenge;
  static deserializeBinaryFromReader(message: WorkerHandshakeChallenge, reader: jspb.BinaryReader): WorkerHandshakeChallenge;
}

export namespace WorkerHandshakeChallenge {
  export type AsObject = {
    requestId: string,
    challengeId: string,
    traceId: string,
    workerId: string,
    agentId: string,
    tenantId: string,
    proofKeyId: string,
    proofAlgorithm: WorkerHandshakeProofAlgorithmMap[keyof WorkerHandshakeProofAlgorithmMap],
    serverKeyId: string,
    audience: string,
    purpose: WorkerHandshakePurposeMap[keyof WorkerHandshakePurposeMap],
    clientNonce: Uint8Array | string,
    serverNonce: Uint8Array | string,
    protocolVersion: number,
    sdkVersion: string,
    issuedAt?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    expiresAt?: google_protobuf_timestamp_pb.Timestamp.AsObject,
  }
}

export class WorkerHandshakeAuthenticate extends jspb.Message {
  hasChallenge(): boolean;
  clearChallenge(): void;
  getChallenge(): WorkerHandshakeChallenge | undefined;
  setChallenge(value?: WorkerHandshakeChallenge): void;

  hasCapabilityHandshake(): boolean;
  clearCapabilityHandshake(): void;
  getCapabilityHandshake(): Handshake | undefined;
  setCapabilityHandshake(value?: Handshake): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): WorkerHandshakeAuthenticate.AsObject;
  static toObject(includeInstance: boolean, msg: WorkerHandshakeAuthenticate): WorkerHandshakeAuthenticate.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: WorkerHandshakeAuthenticate, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): WorkerHandshakeAuthenticate;
  static deserializeBinaryFromReader(message: WorkerHandshakeAuthenticate, reader: jspb.BinaryReader): WorkerHandshakeAuthenticate;
}

export namespace WorkerHandshakeAuthenticate {
  export type AsObject = {
    challenge?: WorkerHandshakeChallenge.AsObject,
    capabilityHandshake?: Handshake.AsObject,
  }
}

export class WorkerHandshakeResult extends jspb.Message {
  hasChallenge(): boolean;
  clearChallenge(): void;
  getChallenge(): WorkerHandshakeChallenge | undefined;
  setChallenge(value?: WorkerHandshakeChallenge): void;

  getAccepted(): boolean;
  setAccepted(value: boolean): void;

  getRejectionReason(): WorkerHandshakeRejectionReasonMap[keyof WorkerHandshakeRejectionReasonMap];
  setRejectionReason(value: WorkerHandshakeRejectionReasonMap[keyof WorkerHandshakeRejectionReasonMap]): void;

  hasTokenExpiresAt(): boolean;
  clearTokenExpiresAt(): void;
  getTokenExpiresAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setTokenExpiresAt(value?: google_protobuf_timestamp_pb.Timestamp): void;

  hasIssuedAt(): boolean;
  clearIssuedAt(): void;
  getIssuedAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setIssuedAt(value?: google_protobuf_timestamp_pb.Timestamp): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): WorkerHandshakeResult.AsObject;
  static toObject(includeInstance: boolean, msg: WorkerHandshakeResult): WorkerHandshakeResult.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: WorkerHandshakeResult, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): WorkerHandshakeResult;
  static deserializeBinaryFromReader(message: WorkerHandshakeResult, reader: jspb.BinaryReader): WorkerHandshakeResult;
}

export namespace WorkerHandshakeResult {
  export type AsObject = {
    challenge?: WorkerHandshakeChallenge.AsObject,
    accepted: boolean,
    rejectionReason: WorkerHandshakeRejectionReasonMap[keyof WorkerHandshakeRejectionReasonMap],
    tokenExpiresAt?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    issuedAt?: google_protobuf_timestamp_pb.Timestamp.AsObject,
  }
}

export interface ComponentRoleMap {
  COMPONENT_ROLE_UNSPECIFIED: 0;
  COMPONENT_ROLE_GATEWAY: 1;
  COMPONENT_ROLE_SCHEDULER: 2;
  COMPONENT_ROLE_WORKER: 3;
  COMPONENT_ROLE_ORCHESTRATOR: 4;
  COMPONENT_ROLE_CONTROLLER: 5;
}

export const ComponentRole: ComponentRoleMap;

export interface WorkerHandshakePurposeMap {
  WORKER_HANDSHAKE_PURPOSE_UNSPECIFIED: 0;
  WORKER_HANDSHAKE_PURPOSE_ISSUE: 1;
  WORKER_HANDSHAKE_PURPOSE_RENEW: 2;
}

export const WorkerHandshakePurpose: WorkerHandshakePurposeMap;

export interface WorkerHandshakeProofAlgorithmMap {
  WORKER_HANDSHAKE_PROOF_ALGORITHM_UNSPECIFIED: 0;
  WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256: 1;
}

export const WorkerHandshakeProofAlgorithm: WorkerHandshakeProofAlgorithmMap;

export interface WorkerHandshakeRejectionReasonMap {
  WORKER_HANDSHAKE_REJECTION_REASON_UNSPECIFIED: 0;
  WORKER_HANDSHAKE_REJECTION_REASON_INVALID_REQUEST: 1;
  WORKER_HANDSHAKE_REJECTION_REASON_AUTHENTICATION_FAILED: 2;
  WORKER_HANDSHAKE_REJECTION_REASON_REPLAY_DETECTED: 3;
  WORKER_HANDSHAKE_REJECTION_REASON_CLOCK_SKEW: 4;
  WORKER_HANDSHAKE_REJECTION_REASON_CHALLENGE_EXPIRED: 5;
  WORKER_HANDSHAKE_REJECTION_REASON_SESSION_REQUIRED: 6;
  WORKER_HANDSHAKE_REJECTION_REASON_SESSION_INVALID: 7;
  WORKER_HANDSHAKE_REJECTION_REASON_UNSUPPORTED_VERSION: 8;
  WORKER_HANDSHAKE_REJECTION_REASON_INTERNAL_ERROR: 9;
}

export const WorkerHandshakeRejectionReason: WorkerHandshakeRejectionReasonMap;

