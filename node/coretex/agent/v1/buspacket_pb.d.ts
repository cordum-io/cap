// package: coretex.agent.v1
// file: coretex/agent/v1/buspacket.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as coretex_agent_v1_job_pb from "../../../coretex/agent/v1/job_pb";
import * as coretex_agent_v1_heartbeat_pb from "../../../coretex/agent/v1/heartbeat_pb";
import * as coretex_agent_v1_alert_pb from "../../../coretex/agent/v1/alert_pb";

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
  getJobRequest(): coretex_agent_v1_job_pb.JobRequest | undefined;
  setJobRequest(value?: coretex_agent_v1_job_pb.JobRequest): void;

  hasJobResult(): boolean;
  clearJobResult(): void;
  getJobResult(): coretex_agent_v1_job_pb.JobResult | undefined;
  setJobResult(value?: coretex_agent_v1_job_pb.JobResult): void;

  hasHeartbeat(): boolean;
  clearHeartbeat(): void;
  getHeartbeat(): coretex_agent_v1_heartbeat_pb.Heartbeat | undefined;
  setHeartbeat(value?: coretex_agent_v1_heartbeat_pb.Heartbeat): void;

  hasAlert(): boolean;
  clearAlert(): void;
  getAlert(): coretex_agent_v1_alert_pb.SystemAlert | undefined;
  setAlert(value?: coretex_agent_v1_alert_pb.SystemAlert): void;

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
    jobRequest?: coretex_agent_v1_job_pb.JobRequest.AsObject,
    jobResult?: coretex_agent_v1_job_pb.JobResult.AsObject,
    heartbeat?: coretex_agent_v1_heartbeat_pb.Heartbeat.AsObject,
    alert?: coretex_agent_v1_alert_pb.SystemAlert.AsObject,
    signature: Uint8Array | string,
  }

  export enum PayloadCase {
    PAYLOAD_NOT_SET = 0,
    JOB_REQUEST = 10,
    JOB_RESULT = 11,
    HEARTBEAT = 12,
    ALERT = 13,
  }
}

