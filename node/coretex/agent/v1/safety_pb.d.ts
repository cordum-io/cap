// package: coretex.agent.v1
// file: coretex/agent/v1/safety.proto

import * as jspb from "google-protobuf";
import * as coretex_agent_v1_job_pb from "../../../coretex/agent/v1/job_pb";

export class PolicyCheckRequest extends jspb.Message {
  getJobId(): string;
  setJobId(value: string): void;

  getTopic(): string;
  setTopic(value: string): void;

  getTenant(): string;
  setTenant(value: string): void;

  getPriority(): coretex_agent_v1_job_pb.JobPriorityMap[keyof coretex_agent_v1_job_pb.JobPriorityMap];
  setPriority(value: coretex_agent_v1_job_pb.JobPriorityMap[keyof coretex_agent_v1_job_pb.JobPriorityMap]): void;

  getEstimatedCost(): number;
  setEstimatedCost(value: number): void;

  hasBudget(): boolean;
  clearBudget(): void;
  getBudget(): coretex_agent_v1_job_pb.Budget | undefined;
  setBudget(value?: coretex_agent_v1_job_pb.Budget): void;

  getPrincipalId(): string;
  setPrincipalId(value: string): void;

  getLabelsMap(): jspb.Map<string, string>;
  clearLabelsMap(): void;
  getMemoryId(): string;
  setMemoryId(value: string): void;

  getEffectiveConfig(): Uint8Array | string;
  getEffectiveConfig_asU8(): Uint8Array;
  getEffectiveConfig_asB64(): string;
  setEffectiveConfig(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PolicyCheckRequest.AsObject;
  static toObject(includeInstance: boolean, msg: PolicyCheckRequest): PolicyCheckRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PolicyCheckRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PolicyCheckRequest;
  static deserializeBinaryFromReader(message: PolicyCheckRequest, reader: jspb.BinaryReader): PolicyCheckRequest;
}

export namespace PolicyCheckRequest {
  export type AsObject = {
    jobId: string,
    topic: string,
    tenant: string,
    priority: coretex_agent_v1_job_pb.JobPriorityMap[keyof coretex_agent_v1_job_pb.JobPriorityMap],
    estimatedCost: number,
    budget?: coretex_agent_v1_job_pb.Budget.AsObject,
    principalId: string,
    labelsMap: Array<[string, string]>,
    memoryId: string,
    effectiveConfig: Uint8Array | string,
  }
}

export class BudgetConstraints extends jspb.Message {
  getMaxRuntimeMs(): number;
  setMaxRuntimeMs(value: number): void;

  getMaxRetries(): number;
  setMaxRetries(value: number): void;

  getMaxArtifactBytes(): number;
  setMaxArtifactBytes(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BudgetConstraints.AsObject;
  static toObject(includeInstance: boolean, msg: BudgetConstraints): BudgetConstraints.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BudgetConstraints, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BudgetConstraints;
  static deserializeBinaryFromReader(message: BudgetConstraints, reader: jspb.BinaryReader): BudgetConstraints;
}

export namespace BudgetConstraints {
  export type AsObject = {
    maxRuntimeMs: number,
    maxRetries: number,
    maxArtifactBytes: number,
  }
}

export class SandboxProfile extends jspb.Message {
  getIsolated(): boolean;
  setIsolated(value: boolean): void;

  clearNetworkAllowlistList(): void;
  getNetworkAllowlistList(): Array<string>;
  setNetworkAllowlistList(value: Array<string>): void;
  addNetworkAllowlist(value: string, index?: number): string;

  clearFsReadOnlyList(): void;
  getFsReadOnlyList(): Array<string>;
  setFsReadOnlyList(value: Array<string>): void;
  addFsReadOnly(value: string, index?: number): string;

  clearFsReadWriteList(): void;
  getFsReadWriteList(): Array<string>;
  setFsReadWriteList(value: Array<string>): void;
  addFsReadWrite(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SandboxProfile.AsObject;
  static toObject(includeInstance: boolean, msg: SandboxProfile): SandboxProfile.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SandboxProfile, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SandboxProfile;
  static deserializeBinaryFromReader(message: SandboxProfile, reader: jspb.BinaryReader): SandboxProfile;
}

export namespace SandboxProfile {
  export type AsObject = {
    isolated: boolean,
    networkAllowlistList: Array<string>,
    fsReadOnlyList: Array<string>,
    fsReadWriteList: Array<string>,
  }
}

export class ToolchainConstraints extends jspb.Message {
  clearAllowedToolsList(): void;
  getAllowedToolsList(): Array<string>;
  setAllowedToolsList(value: Array<string>): void;
  addAllowedTools(value: string, index?: number): string;

  clearAllowedCommandsList(): void;
  getAllowedCommandsList(): Array<string>;
  setAllowedCommandsList(value: Array<string>): void;
  addAllowedCommands(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ToolchainConstraints.AsObject;
  static toObject(includeInstance: boolean, msg: ToolchainConstraints): ToolchainConstraints.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ToolchainConstraints, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ToolchainConstraints;
  static deserializeBinaryFromReader(message: ToolchainConstraints, reader: jspb.BinaryReader): ToolchainConstraints;
}

export namespace ToolchainConstraints {
  export type AsObject = {
    allowedToolsList: Array<string>,
    allowedCommandsList: Array<string>,
  }
}

export class DiffConstraints extends jspb.Message {
  getMaxFiles(): number;
  setMaxFiles(value: number): void;

  getMaxLines(): number;
  setMaxLines(value: number): void;

  clearDenyPathGlobsList(): void;
  getDenyPathGlobsList(): Array<string>;
  setDenyPathGlobsList(value: Array<string>): void;
  addDenyPathGlobs(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DiffConstraints.AsObject;
  static toObject(includeInstance: boolean, msg: DiffConstraints): DiffConstraints.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DiffConstraints, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DiffConstraints;
  static deserializeBinaryFromReader(message: DiffConstraints, reader: jspb.BinaryReader): DiffConstraints;
}

export namespace DiffConstraints {
  export type AsObject = {
    maxFiles: number,
    maxLines: number,
    denyPathGlobsList: Array<string>,
  }
}

export class PolicyConstraints extends jspb.Message {
  hasBudgets(): boolean;
  clearBudgets(): void;
  getBudgets(): BudgetConstraints | undefined;
  setBudgets(value?: BudgetConstraints): void;

  hasSandbox(): boolean;
  clearSandbox(): void;
  getSandbox(): SandboxProfile | undefined;
  setSandbox(value?: SandboxProfile): void;

  hasToolchain(): boolean;
  clearToolchain(): void;
  getToolchain(): ToolchainConstraints | undefined;
  setToolchain(value?: ToolchainConstraints): void;

  hasDiff(): boolean;
  clearDiff(): void;
  getDiff(): DiffConstraints | undefined;
  setDiff(value?: DiffConstraints): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PolicyConstraints.AsObject;
  static toObject(includeInstance: boolean, msg: PolicyConstraints): PolicyConstraints.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PolicyConstraints, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PolicyConstraints;
  static deserializeBinaryFromReader(message: PolicyConstraints, reader: jspb.BinaryReader): PolicyConstraints;
}

export namespace PolicyConstraints {
  export type AsObject = {
    budgets?: BudgetConstraints.AsObject,
    sandbox?: SandboxProfile.AsObject,
    toolchain?: ToolchainConstraints.AsObject,
    diff?: DiffConstraints.AsObject,
  }
}

export class PolicyCheckResponse extends jspb.Message {
  getDecision(): DecisionTypeMap[keyof DecisionTypeMap];
  setDecision(value: DecisionTypeMap[keyof DecisionTypeMap]): void;

  getReason(): string;
  setReason(value: string): void;

  getRedactedContextPtr(): string;
  setRedactedContextPtr(value: string): void;

  getPolicySnapshot(): string;
  setPolicySnapshot(value: string): void;

  getRuleId(): string;
  setRuleId(value: string): void;

  hasConstraints(): boolean;
  clearConstraints(): void;
  getConstraints(): PolicyConstraints | undefined;
  setConstraints(value?: PolicyConstraints): void;

  getApprovalRequired(): boolean;
  setApprovalRequired(value: boolean): void;

  getApprovalRef(): string;
  setApprovalRef(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PolicyCheckResponse.AsObject;
  static toObject(includeInstance: boolean, msg: PolicyCheckResponse): PolicyCheckResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PolicyCheckResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PolicyCheckResponse;
  static deserializeBinaryFromReader(message: PolicyCheckResponse, reader: jspb.BinaryReader): PolicyCheckResponse;
}

export namespace PolicyCheckResponse {
  export type AsObject = {
    decision: DecisionTypeMap[keyof DecisionTypeMap],
    reason: string,
    redactedContextPtr: string,
    policySnapshot: string,
    ruleId: string,
    constraints?: PolicyConstraints.AsObject,
    approvalRequired: boolean,
    approvalRef: string,
  }
}

export class ListSnapshotsRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListSnapshotsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListSnapshotsRequest): ListSnapshotsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListSnapshotsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListSnapshotsRequest;
  static deserializeBinaryFromReader(message: ListSnapshotsRequest, reader: jspb.BinaryReader): ListSnapshotsRequest;
}

export namespace ListSnapshotsRequest {
  export type AsObject = {
  }
}

export class ListSnapshotsResponse extends jspb.Message {
  clearSnapshotsList(): void;
  getSnapshotsList(): Array<string>;
  setSnapshotsList(value: Array<string>): void;
  addSnapshots(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListSnapshotsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ListSnapshotsResponse): ListSnapshotsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListSnapshotsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListSnapshotsResponse;
  static deserializeBinaryFromReader(message: ListSnapshotsResponse, reader: jspb.BinaryReader): ListSnapshotsResponse;
}

export namespace ListSnapshotsResponse {
  export type AsObject = {
    snapshotsList: Array<string>,
  }
}

export interface DecisionTypeMap {
  DECISION_TYPE_UNSPECIFIED: 0;
  DECISION_TYPE_ALLOW: 1;
  DECISION_TYPE_DENY: 2;
  DECISION_TYPE_REQUIRE_HUMAN: 3;
  DECISION_TYPE_THROTTLE: 4;
}

export const DecisionType: DecisionTypeMap;

