// package: coretex.agent.v1
// file: coretex/agent/v1/heartbeat.proto

import * as jspb from "google-protobuf";

export class Heartbeat extends jspb.Message {
  getWorkerId(): string;
  setWorkerId(value: string): void;

  getRegion(): string;
  setRegion(value: string): void;

  getType(): string;
  setType(value: string): void;

  getCpuLoad(): number;
  setCpuLoad(value: number): void;

  getGpuUtilization(): number;
  setGpuUtilization(value: number): void;

  getActiveJobs(): number;
  setActiveJobs(value: number): void;

  clearCapabilitiesList(): void;
  getCapabilitiesList(): Array<string>;
  setCapabilitiesList(value: Array<string>): void;
  addCapabilities(value: string, index?: number): string;

  getPool(): string;
  setPool(value: string): void;

  getMaxParallelJobs(): number;
  setMaxParallelJobs(value: number): void;

  getLabelsMap(): jspb.Map<string, string>;
  clearLabelsMap(): void;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Heartbeat.AsObject;
  static toObject(includeInstance: boolean, msg: Heartbeat): Heartbeat.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Heartbeat, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Heartbeat;
  static deserializeBinaryFromReader(message: Heartbeat, reader: jspb.BinaryReader): Heartbeat;
}

export namespace Heartbeat {
  export type AsObject = {
    workerId: string,
    region: string,
    type: string,
    cpuLoad: number,
    gpuUtilization: number,
    activeJobs: number,
    capabilitiesList: Array<string>,
    pool: string,
    maxParallelJobs: number,
    labelsMap: Array<[string, string]>,
  }
}

