// package: cordum.agent.v1
// file: cordum/agent/v1/job.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class IdentityBinding extends jspb.Message {
  getTenantId(): string;
  setTenantId(value: string): void;

  getPrincipalId(): string;
  setPrincipalId(value: string): void;

  getActorId(): string;
  setActorId(value: string): void;

  getDelegationId(): string;
  setDelegationId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IdentityBinding.AsObject;
  static toObject(includeInstance: boolean, msg: IdentityBinding): IdentityBinding.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IdentityBinding, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IdentityBinding;
  static deserializeBinaryFromReader(message: IdentityBinding, reader: jspb.BinaryReader): IdentityBinding;
}

export namespace IdentityBinding {
  export type AsObject = {
    tenantId: string,
    principalId: string,
    actorId: string,
    delegationId: string,
  }
}

export class DispatchIdentity extends jspb.Message {
  getDispatchId(): string;
  setDispatchId(value: string): void;

  getAttempt(): number;
  setAttempt(value: number): void;

  getAssignedWorkerId(): string;
  setAssignedWorkerId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DispatchIdentity.AsObject;
  static toObject(includeInstance: boolean, msg: DispatchIdentity): DispatchIdentity.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DispatchIdentity, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DispatchIdentity;
  static deserializeBinaryFromReader(message: DispatchIdentity, reader: jspb.BinaryReader): DispatchIdentity;
}

export namespace DispatchIdentity {
  export type AsObject = {
    dispatchId: string,
    attempt: number,
    assignedWorkerId: string,
  }
}

export class ResourceRef extends jspb.Message {
  getResolverId(): string;
  setResolverId(value: string): void;

  getUri(): string;
  setUri(value: string): void;

  getSha256(): Uint8Array | string;
  getSha256_asU8(): Uint8Array;
  getSha256_asB64(): string;
  setSha256(value: Uint8Array | string): void;

  getMediaType(): string;
  setMediaType(value: string): void;

  getSizeBytes(): number;
  setSizeBytes(value: number): void;

  hasExpiresAt(): boolean;
  clearExpiresAt(): void;
  getExpiresAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setExpiresAt(value?: google_protobuf_timestamp_pb.Timestamp): void;

  getPurpose(): string;
  setPurpose(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ResourceRef.AsObject;
  static toObject(includeInstance: boolean, msg: ResourceRef): ResourceRef.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ResourceRef, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ResourceRef;
  static deserializeBinaryFromReader(message: ResourceRef, reader: jspb.BinaryReader): ResourceRef;
}

export namespace ResourceRef {
  export type AsObject = {
    resolverId: string,
    uri: string,
    sha256: Uint8Array | string,
    mediaType: string,
    sizeBytes: number,
    expiresAt?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    purpose: string,
  }
}

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

export class Compensation extends jspb.Message {
  getTopic(): string;
  setTopic(value: string): void;

  getContextPtr(): string;
  setContextPtr(value: string): void;

  getPriority(): JobPriorityMap[keyof JobPriorityMap];
  setPriority(value: JobPriorityMap[keyof JobPriorityMap]): void;

  getAdapterId(): string;
  setAdapterId(value: string): void;

  getEnvMap(): jspb.Map<string, string>;
  clearEnvMap(): void;
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

  hasIdentity(): boolean;
  clearIdentity(): void;
  getIdentity(): IdentityBinding | undefined;
  setIdentity(value?: IdentityBinding): void;

  hasDispatch(): boolean;
  clearDispatch(): void;
  getDispatch(): DispatchIdentity | undefined;
  setDispatch(value?: DispatchIdentity): void;

  hasContextRef(): boolean;
  clearContextRef(): void;
  getContextRef(): ResourceRef | undefined;
  setContextRef(value?: ResourceRef): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Compensation.AsObject;
  static toObject(includeInstance: boolean, msg: Compensation): Compensation.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Compensation, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Compensation;
  static deserializeBinaryFromReader(message: Compensation, reader: jspb.BinaryReader): Compensation;
}

export namespace Compensation {
  export type AsObject = {
    topic: string,
    contextPtr: string,
    priority: JobPriorityMap[keyof JobPriorityMap],
    adapterId: string,
    envMap: Array<[string, string]>,
    memoryId: string,
    contextHints?: ContextHints.AsObject,
    budget?: Budget.AsObject,
    tenantId: string,
    principalId: string,
    labelsMap: Array<[string, string]>,
    meta?: JobMetadata.AsObject,
    identity?: IdentityBinding.AsObject,
    dispatch?: DispatchIdentity.AsObject,
    contextRef?: ResourceRef.AsObject,
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

  hasCompensation(): boolean;
  clearCompensation(): void;
  getCompensation(): Compensation | undefined;
  setCompensation(value?: Compensation): void;

  hasIdentity(): boolean;
  clearIdentity(): void;
  getIdentity(): IdentityBinding | undefined;
  setIdentity(value?: IdentityBinding): void;

  hasDispatch(): boolean;
  clearDispatch(): void;
  getDispatch(): DispatchIdentity | undefined;
  setDispatch(value?: DispatchIdentity): void;

  hasContextRef(): boolean;
  clearContextRef(): void;
  getContextRef(): ResourceRef | undefined;
  setContextRef(value?: ResourceRef): void;

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
    compensation?: Compensation.AsObject,
    identity?: IdentityBinding.AsObject,
    dispatch?: DispatchIdentity.AsObject,
    contextRef?: ResourceRef.AsObject,
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

  clearArtifactPtrsList(): void;
  getArtifactPtrsList(): Array<string>;
  setArtifactPtrsList(value: Array<string>): void;
  addArtifactPtrs(value: string, index?: number): string;

  getErrorCodeEnum(): ErrorCodeMap[keyof ErrorCodeMap];
  setErrorCodeEnum(value: ErrorCodeMap[keyof ErrorCodeMap]): void;

  hasDispatch(): boolean;
  clearDispatch(): void;
  getDispatch(): DispatchIdentity | undefined;
  setDispatch(value?: DispatchIdentity): void;

  hasIdentity(): boolean;
  clearIdentity(): void;
  getIdentity(): IdentityBinding | undefined;
  setIdentity(value?: IdentityBinding): void;

  hasResultRef(): boolean;
  clearResultRef(): void;
  getResultRef(): ResourceRef | undefined;
  setResultRef(value?: ResourceRef): void;

  clearArtifactRefsList(): void;
  getArtifactRefsList(): Array<ResourceRef>;
  setArtifactRefsList(value: Array<ResourceRef>): void;
  addArtifactRefs(value?: ResourceRef, index?: number): ResourceRef;

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
    artifactPtrsList: Array<string>,
    errorCodeEnum: ErrorCodeMap[keyof ErrorCodeMap],
    dispatch?: DispatchIdentity.AsObject,
    identity?: IdentityBinding.AsObject,
    resultRef?: ResourceRef.AsObject,
    artifactRefsList: Array<ResourceRef.AsObject>,
  }
}

export class JobProgress extends jspb.Message {
  getJobId(): string;
  setJobId(value: string): void;

  getStepId(): string;
  setStepId(value: string): void;

  getPercent(): number;
  setPercent(value: number): void;

  getMessage(): string;
  setMessage(value: string): void;

  getResultPtr(): string;
  setResultPtr(value: string): void;

  clearArtifactPtrsList(): void;
  getArtifactPtrsList(): Array<string>;
  setArtifactPtrsList(value: Array<string>): void;
  addArtifactPtrs(value: string, index?: number): string;

  getStatus(): JobStatusMap[keyof JobStatusMap];
  setStatus(value: JobStatusMap[keyof JobStatusMap]): void;

  hasDispatch(): boolean;
  clearDispatch(): void;
  getDispatch(): DispatchIdentity | undefined;
  setDispatch(value?: DispatchIdentity): void;

  hasIdentity(): boolean;
  clearIdentity(): void;
  getIdentity(): IdentityBinding | undefined;
  setIdentity(value?: IdentityBinding): void;

  hasResultRef(): boolean;
  clearResultRef(): void;
  getResultRef(): ResourceRef | undefined;
  setResultRef(value?: ResourceRef): void;

  clearArtifactRefsList(): void;
  getArtifactRefsList(): Array<ResourceRef>;
  setArtifactRefsList(value: Array<ResourceRef>): void;
  addArtifactRefs(value?: ResourceRef, index?: number): ResourceRef;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JobProgress.AsObject;
  static toObject(includeInstance: boolean, msg: JobProgress): JobProgress.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JobProgress, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JobProgress;
  static deserializeBinaryFromReader(message: JobProgress, reader: jspb.BinaryReader): JobProgress;
}

export namespace JobProgress {
  export type AsObject = {
    jobId: string,
    stepId: string,
    percent: number,
    message: string,
    resultPtr: string,
    artifactPtrsList: Array<string>,
    status: JobStatusMap[keyof JobStatusMap],
    dispatch?: DispatchIdentity.AsObject,
    identity?: IdentityBinding.AsObject,
    resultRef?: ResourceRef.AsObject,
    artifactRefsList: Array<ResourceRef.AsObject>,
  }
}

export class JobCancel extends jspb.Message {
  getJobId(): string;
  setJobId(value: string): void;

  getReason(): string;
  setReason(value: string): void;

  getRequestedBy(): string;
  setRequestedBy(value: string): void;

  hasDispatch(): boolean;
  clearDispatch(): void;
  getDispatch(): DispatchIdentity | undefined;
  setDispatch(value?: DispatchIdentity): void;

  hasIdentity(): boolean;
  clearIdentity(): void;
  getIdentity(): IdentityBinding | undefined;
  setIdentity(value?: IdentityBinding): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JobCancel.AsObject;
  static toObject(includeInstance: boolean, msg: JobCancel): JobCancel.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JobCancel, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JobCancel;
  static deserializeBinaryFromReader(message: JobCancel, reader: jspb.BinaryReader): JobCancel;
}

export namespace JobCancel {
  export type AsObject = {
    jobId: string,
    reason: string,
    requestedBy: string,
    dispatch?: DispatchIdentity.AsObject,
    identity?: IdentityBinding.AsObject,
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
  JOB_STATUS_FAILED_RETRYABLE: 10;
  JOB_STATUS_FAILED_FATAL: 11;
}

export const JobStatus: JobStatusMap;

export interface ActorTypeMap {
  ACTOR_TYPE_UNSPECIFIED: 0;
  ACTOR_TYPE_HUMAN: 1;
  ACTOR_TYPE_SERVICE: 2;
}

export const ActorType: ActorTypeMap;

export interface ErrorCodeMap {
  ERROR_CODE_UNSPECIFIED: 0;
  ERROR_CODE_PROTOCOL_VERSION_MISMATCH: 100;
  ERROR_CODE_PROTOCOL_MALFORMED_PACKET: 101;
  ERROR_CODE_PROTOCOL_UNKNOWN_PAYLOAD: 102;
  ERROR_CODE_PROTOCOL_SIGNATURE_INVALID: 103;
  ERROR_CODE_PROTOCOL_SIGNATURE_MISSING: 104;
  ERROR_CODE_JOB_TIMEOUT: 200;
  ERROR_CODE_JOB_RESOURCE_EXHAUSTED: 201;
  ERROR_CODE_JOB_PERMISSION_DENIED: 202;
  ERROR_CODE_JOB_INVALID_INPUT: 203;
  ERROR_CODE_JOB_NOT_FOUND: 204;
  ERROR_CODE_JOB_DUPLICATE: 205;
  ERROR_CODE_JOB_WORKER_UNAVAILABLE: 206;
  ERROR_CODE_SAFETY_DENIED: 300;
  ERROR_CODE_SAFETY_POLICY_VIOLATION: 301;
  ERROR_CODE_SAFETY_RISK_TAG_BLOCKED: 302;
  ERROR_CODE_TRANSPORT_PUBLISH_FAILED: 400;
  ERROR_CODE_TRANSPORT_SUBSCRIBE_FAILED: 401;
  ERROR_CODE_TRANSPORT_CONNECTION_LOST: 402;
}

export const ErrorCode: ErrorCodeMap;

