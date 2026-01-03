// package: coretex.agent.v1
// file: coretex/agent/v1/alert.proto

import * as jspb from "google-protobuf";

export class SystemAlert extends jspb.Message {
  getLevel(): string;
  setLevel(value: string): void;

  getMessage(): string;
  setMessage(value: string): void;

  getComponent(): string;
  setComponent(value: string): void;

  getCode(): string;
  setCode(value: string): void;

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
  }
}

