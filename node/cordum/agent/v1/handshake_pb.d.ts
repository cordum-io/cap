// package: cordum.agent.v1
// file: cordum/agent/v1/handshake.proto

import * as jspb from "google-protobuf";

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

