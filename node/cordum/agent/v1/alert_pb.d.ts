// package: cordum.agent.v1
// file: cordum/agent/v1/alert.proto

import * as jspb from "google-protobuf";
import * as cordum_agent_v1_job_pb from "../../../cordum/agent/v1/job_pb";

export class SystemAlert extends jspb.Message {
  getLevel(): string;
  setLevel(value: string): void;

  getMessage(): string;
  setMessage(value: string): void;

  getComponent(): string;
  setComponent(value: string): void;

  getCode(): string;
  setCode(value: string): void;

  getSeverity(): AlertSeverityMap[keyof AlertSeverityMap];
  setSeverity(value: AlertSeverityMap[keyof AlertSeverityMap]): void;

  getErrorCodeEnum(): cordum_agent_v1_job_pb.ErrorCodeMap[keyof cordum_agent_v1_job_pb.ErrorCodeMap];
  setErrorCodeEnum(value: cordum_agent_v1_job_pb.ErrorCodeMap[keyof cordum_agent_v1_job_pb.ErrorCodeMap]): void;

  getSourceComponent(): string;
  setSourceComponent(value: string): void;

  getDetailsMap(): jspb.Map<string, string>;
  clearDetailsMap(): void;
  getTraceId(): string;
  setTraceId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SystemAlert.AsObject;
  static toObject(includeInstance: boolean, msg: SystemAlert): SystemAlert.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SystemAlert, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SystemAlert;
  static deserializeBinaryFromReader(message: SystemAlert, reader: jspb.BinaryReader): SystemAlert;
}

export namespace SystemAlert {
  export type AsObject = {
    level: string,
    message: string,
    component: string,
    code: string,
    severity: AlertSeverityMap[keyof AlertSeverityMap],
    errorCodeEnum: cordum_agent_v1_job_pb.ErrorCodeMap[keyof cordum_agent_v1_job_pb.ErrorCodeMap],
    sourceComponent: string,
    detailsMap: Array<[string, string]>,
    traceId: string,
  }
}

export interface AlertSeverityMap {
  ALERT_SEVERITY_UNSPECIFIED: 0;
  ALERT_SEVERITY_INFO: 1;
  ALERT_SEVERITY_WARNING: 2;
  ALERT_SEVERITY_ERROR: 3;
  ALERT_SEVERITY_CRITICAL: 4;
}

export const AlertSeverity: AlertSeverityMap;

