// package: cordum.agent.v1
// file: cordum/agent/v1/buspacket.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as cordum_agent_v1_job_pb from "../../../cordum/agent/v1/job_pb";
import * as cordum_agent_v1_heartbeat_pb from "../../../cordum/agent/v1/heartbeat_pb";
import * as cordum_agent_v1_alert_pb from "../../../cordum/agent/v1/alert_pb";

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

  getSignature(): Uint8Array | string;
  getSignature_asU8(): Uint8Array;
  getSignature_asB64(): string;
  setSignature(value: Uint8Array | string): void;

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
    jobRequest?: cordum_agent_v1_job_pb.JobRequest.AsObject,
    jobResult?: cordum_agent_v1_job_pb.JobResult.AsObject,
    heartbeat?: cordum_agent_v1_heartbeat_pb.Heartbeat.AsObject,
    alert?: cordum_agent_v1_alert_pb.SystemAlert.AsObject,
    jobProgress?: cordum_agent_v1_job_pb.JobProgress.AsObject,
    jobCancel?: cordum_agent_v1_job_pb.JobCancel.AsObject,
    signature: Uint8Array | string,
  }

  export enum PayloadCase {
    PAYLOAD_NOT_SET = 0,
    JOB_REQUEST = 10,
    JOB_RESULT = 11,
    HEARTBEAT = 12,
    ALERT = 13,
    JOB_PROGRESS = 15,
    JOB_CANCEL = 16,
  }
}

