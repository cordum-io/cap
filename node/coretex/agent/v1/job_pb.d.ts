// package: coretex.agent.v1
// file: coretex/agent/v1/job.proto

import * as jspb from "google-protobuf";

export class ContextHints extends jspb.Message {
  getMaxInputTokens(): number;
  setMaxInputTokens(value: number): void;

  getAllowSummarization(): boolean;
  setAllowSummarization(value: boolean): void;

  getAllowRetrieval(): boolean;
  setAllowRetrieval(value: boolean): void;

  clearTagsList(): void;
  getTagsList(): Array<string>;
  setTagsList(value: Array<string>): void;
  addTags(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ContextHints.AsObject;
  static toObject(includeInstance: boolean, msg: ContextHints): ContextHints.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ContextHints, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ContextHints;
  static deserializeBinaryFromReader(message: ContextHints, reader: jspb.BinaryReader): ContextHints;
}

export namespace ContextHints {
  export type AsObject = {
    maxInputTokens: number,
    allowSummarization: boolean,
    allowRetrieval: boolean,
    tagsList: Array<string>,
  }
}

export class Budget extends jspb.Message {
  getMaxInputTokens(): number;
  setMaxInputTokens(value: number): void;

  getMaxOutputTokens(): number;
  setMaxOutputTokens(value: number): void;

  getMaxTotalTokens(): number;
  setMaxTotalTokens(value: number): void;

  getDeadlineMs(): number;
  setDeadlineMs(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Budget.AsObject;
  static toObject(includeInstance: boolean, msg: Budget): Budget.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Budget, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Budget;
  static deserializeBinaryFromReader(message: Budget, reader: jspb.BinaryReader): Budget;
}

export namespace Budget {
  export type AsObject = {
    maxInputTokens: number,
    maxOutputTokens: number,
    maxTotalTokens: number,
    deadlineMs: number,
  }
}

export class JobMetadata extends jspb.Message {
  getTenantId(): string;
  setTenantId(value: string): void;

  getActorId(): string;
  setActorId(value: string): void;

  getActorType(): ActorTypeMap[keyof ActorTypeMap];
  setActorType(value: ActorTypeMap[keyof ActorTypeMap]): void;

  getIdempotencyKey(): string;
  setIdempotencyKey(value: string): void;

  getCapability(): string;
  setCapability(value: string): void;

  clearRiskTagsList(): void;
  getRiskTagsList(): Array<string>;
  setRiskTagsList(value: Array<string>): void;
  addRiskTags(value: string, index?: number): string;

  clearRequiresList(): void;
  getRequiresList(): Array<string>;
  setRequiresList(value: Array<string>): void;
  addRequires(value: string, index?: number): string;

  getPackId(): string;
  setPackId(value: string): void;

  getLabelsMap(): jspb.Map<string, string>;
  clearLabelsMap(): void;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JobMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: JobMetadata): JobMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JobMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JobMetadata;
  static deserializeBinaryFromReader(message: JobMetadata, reader: jspb.BinaryReader): JobMetadata;
}

export namespace JobMetadata {
  export type AsObject = {
    tenantId: string,
    actorId: string,
    actorType: ActorTypeMap[keyof ActorTypeMap],
    idempotencyKey: string,
    capability: string,
    riskTagsList: Array<string>,
    requiresList: Array<string>,
    packId: string,
    labelsMap: Array<[string, string]>,
  }
}

export class JobRequest extends jspb.Message {
  getJobId(): string;
  setJobId(value: string): void;

  getTopic(): string;
  setTopic(value: string): void;

  getPriority(): JobPriorityMap[keyof JobPriorityMap];
  setPriority(value: JobPriorityMap[keyof JobPriorityMap]): void;

  getContextPtr(): string;
  setContextPtr(value: string): void;

  getAdapterId(): string;
  setAdapterId(value: string): void;

  getEnvMap(): jspb.Map<string, string>;
  clearEnvMap(): void;
  getParentJobId(): string;
  setParentJobId(value: string): void;

  getWorkflowId(): string;
  setWorkflowId(value: string): void;

  getStepIndex(): number;
  setStepIndex(value: number): void;

  getMemoryId(): string;
  setMemoryId(value: string): void;

  hasContextHints(): boolean;
  clearContextHints(): void;
  getContextHints(): ContextHints | undefined;
  setContextHints(value?: ContextHints): void;

  hasBudget(): boolean;
  clearBudget(): void;
  getBudget(): Budget | undefined;
  setBudget(value?: Budget): void;

  getTenantId(): string;
  setTenantId(value: string): void;

  getPrincipalId(): string;
  setPrincipalId(value: string): void;

  getLabelsMap(): jspb.Map<string, string>;
  clearLabelsMap(): void;
  hasMeta(): boolean;
  clearMeta(): void;
  getMeta(): JobMetadata | undefined;
  setMeta(value?: JobMetadata): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JobRequest.AsObject;
  static toObject(includeInstance: boolean, msg: JobRequest): JobRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JobRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JobRequest;
  static deserializeBinaryFromReader(message: JobRequest, reader: jspb.BinaryReader): JobRequest;
}

export namespace JobRequest {
  export type AsObject = {
    jobId: string,
    topic: string,
    priority: JobPriorityMap[keyof JobPriorityMap],
    contextPtr: string,
    adapterId: string,
    envMap: Array<[string, string]>,
    parentJobId: string,
    workflowId: string,
    stepIndex: number,
    memoryId: string,
    contextHints?: ContextHints.AsObject,
    budget?: Budget.AsObject,
    tenantId: string,
    principalId: string,
    labelsMap: Array<[string, string]>,
    meta?: JobMetadata.AsObject,
  }
}

export class JobResult extends jspb.Message {
  getJobId(): string;
  setJobId(value: string): void;

  getStatus(): JobStatusMap[keyof JobStatusMap];
  setStatus(value: JobStatusMap[keyof JobStatusMap]): void;

  getResultPtr(): string;
  setResultPtr(value: string): void;

  getWorkerId(): string;
  setWorkerId(value: string): void;

  getExecutionMs(): number;
  setExecutionMs(value: number): void;

  getErrorCode(): string;
  setErrorCode(value: string): void;

  getErrorMessage(): string;
  setErrorMessage(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JobResult.AsObject;
  static toObject(includeInstance: boolean, msg: JobResult): JobResult.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JobResult, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JobResult;
  static deserializeBinaryFromReader(message: JobResult, reader: jspb.BinaryReader): JobResult;
}

export namespace JobResult {
  export type AsObject = {
    jobId: string,
    status: JobStatusMap[keyof JobStatusMap],
    resultPtr: string,
    workerId: string,
    executionMs: number,
    errorCode: string,
    errorMessage: string,
  }
}

export interface JobPriorityMap {
  JOB_PRIORITY_UNSPECIFIED: 0;
  JOB_PRIORITY_INTERACTIVE: 1;
  JOB_PRIORITY_BATCH: 2;
  JOB_PRIORITY_CRITICAL: 3;
}

export const JobPriority: JobPriorityMap;

export interface JobStatusMap {
  JOB_STATUS_UNSPECIFIED: 0;
  JOB_STATUS_PENDING: 1;
  JOB_STATUS_SCHEDULED: 2;
  JOB_STATUS_DISPATCHED: 3;
  JOB_STATUS_RUNNING: 4;
  JOB_STATUS_SUCCEEDED: 5;
  JOB_STATUS_FAILED: 6;
  JOB_STATUS_CANCELLED: 7;
  JOB_STATUS_DENIED: 8;
  JOB_STATUS_TIMEOUT: 9;
}

export const JobStatus: JobStatusMap;

export interface ActorTypeMap {
  ACTOR_TYPE_UNSPECIFIED: 0;
  ACTOR_TYPE_HUMAN: 1;
  ACTOR_TYPE_SERVICE: 2;
}

export const ActorType: ActorTypeMap;

