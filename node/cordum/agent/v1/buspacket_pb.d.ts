// package: cordum.agent.v1
// file: cordum/agent/v1/buspacket.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as cordum_agent_v1_job_pb from "../../../cordum/agent/v1/job_pb";
import * as cordum_agent_v1_heartbeat_pb from "../../../cordum/agent/v1/heartbeat_pb";
import * as cordum_agent_v1_alert_pb from "../../../cordum/agent/v1/alert_pb";
import * as cordum_agent_v1_handshake_pb from "../../../cordum/agent/v1/handshake_pb";

export class BusPacket extends jspb.Message {
  getTraceId(): string;
  setTraceId(value: string): void;

  getSenderId(): string;
  setSenderId(value: string): void;

  hasCreatedAt(): boolean;
  clearCreatedAt(): void;
  getCreatedAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setCreatedAt(value?: google_protobuf_timestamp_pb.Timestamp): void;

  getProtocolVersion(): number;
  setProtocolVersion(value: number): void;

  hasSignatureMetadata(): boolean;
  clearSignatureMetadata(): void;
  getSignatureMetadata(): SignatureMetadata | undefined;
  setSignatureMetadata(value?: SignatureMetadata): void;

  hasIdentity(): boolean;
  clearIdentity(): void;
  getIdentity(): cordum_agent_v1_job_pb.IdentityBinding | undefined;
  setIdentity(value?: cordum_agent_v1_job_pb.IdentityBinding): void;

  hasJobRequest(): boolean;
  clearJobRequest(): void;
  getJobRequest(): cordum_agent_v1_job_pb.JobRequest | undefined;
  setJobRequest(value?: cordum_agent_v1_job_pb.JobRequest): void;

  hasJobResult(): boolean;
  clearJobResult(): void;
  getJobResult(): cordum_agent_v1_job_pb.JobResult | undefined;
  setJobResult(value?: cordum_agent_v1_job_pb.JobResult): void;

  hasHeartbeat(): boolean;
  clearHeartbeat(): void;
  getHeartbeat(): cordum_agent_v1_heartbeat_pb.Heartbeat | undefined;
  setHeartbeat(value?: cordum_agent_v1_heartbeat_pb.Heartbeat): void;

  hasAlert(): boolean;
  clearAlert(): void;
  getAlert(): cordum_agent_v1_alert_pb.SystemAlert | undefined;
  setAlert(value?: cordum_agent_v1_alert_pb.SystemAlert): void;

  hasJobProgress(): boolean;
  clearJobProgress(): void;
  getJobProgress(): cordum_agent_v1_job_pb.JobProgress | undefined;
  setJobProgress(value?: cordum_agent_v1_job_pb.JobProgress): void;

  hasJobCancel(): boolean;
  clearJobCancel(): void;
  getJobCancel(): cordum_agent_v1_job_pb.JobCancel | undefined;
  setJobCancel(value?: cordum_agent_v1_job_pb.JobCancel): void;

  hasHandshake(): boolean;
  clearHandshake(): void;
  getHandshake(): cordum_agent_v1_handshake_pb.Handshake | undefined;
  setHandshake(value?: cordum_agent_v1_handshake_pb.Handshake): void;

  hasWorkerHandshakeChallengeRequest(): boolean;
  clearWorkerHandshakeChallengeRequest(): void;
  getWorkerHandshakeChallengeRequest(): cordum_agent_v1_handshake_pb.WorkerHandshakeChallengeRequest | undefined;
  setWorkerHandshakeChallengeRequest(value?: cordum_agent_v1_handshake_pb.WorkerHandshakeChallengeRequest): void;

  hasWorkerHandshakeChallenge(): boolean;
  clearWorkerHandshakeChallenge(): void;
  getWorkerHandshakeChallenge(): cordum_agent_v1_handshake_pb.WorkerHandshakeChallenge | undefined;
  setWorkerHandshakeChallenge(value?: cordum_agent_v1_handshake_pb.WorkerHandshakeChallenge): void;

  hasWorkerHandshakeAuthenticate(): boolean;
  clearWorkerHandshakeAuthenticate(): void;
  getWorkerHandshakeAuthenticate(): cordum_agent_v1_handshake_pb.WorkerHandshakeAuthenticate | undefined;
  setWorkerHandshakeAuthenticate(value?: cordum_agent_v1_handshake_pb.WorkerHandshakeAuthenticate): void;

  hasWorkerHandshakeResult(): boolean;
  clearWorkerHandshakeResult(): void;
  getWorkerHandshakeResult(): cordum_agent_v1_handshake_pb.WorkerHandshakeResult | undefined;
  setWorkerHandshakeResult(value?: cordum_agent_v1_handshake_pb.WorkerHandshakeResult): void;

  getSignature(): Uint8Array | string;
  getSignature_asU8(): Uint8Array;
  getSignature_asB64(): string;
  setSignature(value: Uint8Array | string): void;

  getAuthToken(): string;
  setAuthToken(value: string): void;

  getPayloadCase(): BusPacket.PayloadCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BusPacket.AsObject;
  static toObject(includeInstance: boolean, msg: BusPacket): BusPacket.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BusPacket, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BusPacket;
  static deserializeBinaryFromReader(message: BusPacket, reader: jspb.BinaryReader): BusPacket;
}

export namespace BusPacket {
  export type AsObject = {
    traceId: string,
    senderId: string,
    createdAt?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    protocolVersion: number,
    signatureMetadata?: SignatureMetadata.AsObject,
    identity?: cordum_agent_v1_job_pb.IdentityBinding.AsObject,
    jobRequest?: cordum_agent_v1_job_pb.JobRequest.AsObject,
    jobResult?: cordum_agent_v1_job_pb.JobResult.AsObject,
    heartbeat?: cordum_agent_v1_heartbeat_pb.Heartbeat.AsObject,
    alert?: cordum_agent_v1_alert_pb.SystemAlert.AsObject,
    jobProgress?: cordum_agent_v1_job_pb.JobProgress.AsObject,
    jobCancel?: cordum_agent_v1_job_pb.JobCancel.AsObject,
    handshake?: cordum_agent_v1_handshake_pb.Handshake.AsObject,
    workerHandshakeChallengeRequest?: cordum_agent_v1_handshake_pb.WorkerHandshakeChallengeRequest.AsObject,
    workerHandshakeChallenge?: cordum_agent_v1_handshake_pb.WorkerHandshakeChallenge.AsObject,
    workerHandshakeAuthenticate?: cordum_agent_v1_handshake_pb.WorkerHandshakeAuthenticate.AsObject,
    workerHandshakeResult?: cordum_agent_v1_handshake_pb.WorkerHandshakeResult.AsObject,
    signature: Uint8Array | string,
    authToken: string,
  }

  export enum PayloadCase {
    PAYLOAD_NOT_SET = 0,
    JOB_REQUEST = 10,
    JOB_RESULT = 11,
    HEARTBEAT = 12,
    ALERT = 13,
    JOB_PROGRESS = 15,
    JOB_CANCEL = 16,
    HANDSHAKE = 17,
    WORKER_HANDSHAKE_CHALLENGE_REQUEST = 19,
    WORKER_HANDSHAKE_CHALLENGE = 20,
    WORKER_HANDSHAKE_AUTHENTICATE = 21,
    WORKER_HANDSHAKE_RESULT = 22,
  }
}

export class SignatureMetadata extends jspb.Message {
  getProfileVersion(): string;
  setProfileVersion(value: string): void;

  getAlgorithm(): string;
  setAlgorithm(value: string): void;

  getMessageId(): Uint8Array | string;
  getMessageId_asU8(): Uint8Array;
  getMessageId_asB64(): string;
  setMessageId(value: Uint8Array | string): void;

  getAudience(): string;
  setAudience(value: string): void;

  hasExpiresAt(): boolean;
  clearExpiresAt(): void;
  getExpiresAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setExpiresAt(value?: google_protobuf_timestamp_pb.Timestamp): void;

  getKeyId(): string;
  setKeyId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SignatureMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: SignatureMetadata): SignatureMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SignatureMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SignatureMetadata;
  static deserializeBinaryFromReader(message: SignatureMetadata, reader: jspb.BinaryReader): SignatureMetadata;
}

export namespace SignatureMetadata {
  export type AsObject = {
    profileVersion: string,
    algorithm: string,
    messageId: Uint8Array | string,
    audience: string,
    expiresAt?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    keyId: string,
  }
}

