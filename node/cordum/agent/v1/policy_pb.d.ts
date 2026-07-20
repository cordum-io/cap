// package: cordum.agent.v1
// file: cordum/agent/v1/policy.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as cordum_agent_v1_safety_pb from "../../../cordum/agent/v1/safety_pb";

export class RuleScope extends jspb.Message {
  getKind(): RuleScopeKindMap[keyof RuleScopeKindMap];
  setKind(value: RuleScopeKindMap[keyof RuleScopeKindMap]): void;

  getValue(): string;
  setValue(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RuleScope.AsObject;
  static toObject(includeInstance: boolean, msg: RuleScope): RuleScope.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: RuleScope, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RuleScope;
  static deserializeBinaryFromReader(message: RuleScope, reader: jspb.BinaryReader): RuleScope;
}

export namespace RuleScope {
  export type AsObject = {
    kind: RuleScopeKindMap[keyof RuleScopeKindMap],
    value: string,
  }
}

export class AuditMetadata extends jspb.Message {
  hasCreatedAt(): boolean;
  clearCreatedAt(): void;
  getCreatedAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setCreatedAt(value?: google_protobuf_timestamp_pb.Timestamp): void;

  getCreatedBy(): string;
  setCreatedBy(value: string): void;

  hasUpdatedAt(): boolean;
  clearUpdatedAt(): void;
  getUpdatedAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setUpdatedAt(value?: google_protobuf_timestamp_pb.Timestamp): void;

  getUpdatedBy(): string;
  setUpdatedBy(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AuditMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: AuditMetadata): AuditMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: AuditMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AuditMetadata;
  static deserializeBinaryFromReader(message: AuditMetadata, reader: jspb.BinaryReader): AuditMetadata;
}

export namespace AuditMetadata {
  export type AsObject = {
    createdAt?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    createdBy: string,
    updatedAt?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    updatedBy: string,
  }
}

export class Rule extends jspb.Message {
  getId(): string;
  setId(value: string): void;

  getName(): string;
  setName(value: string): void;

  getType(): RuleTypeMap[keyof RuleTypeMap];
  setType(value: RuleTypeMap[keyof RuleTypeMap]): void;

  hasScope(): boolean;
  clearScope(): void;
  getScope(): RuleScope | undefined;
  setScope(value?: RuleScope): void;

  getStatus(): RuleStatusMap[keyof RuleStatusMap];
  setStatus(value: RuleStatusMap[keyof RuleStatusMap]): void;

  getVersion(): string;
  setVersion(value: string): void;

  hasAudit(): boolean;
  clearAudit(): void;
  getAudit(): AuditMetadata | undefined;
  setAudit(value?: AuditMetadata): void;

  hasMatch(): boolean;
  clearMatch(): void;
  getMatch(): google_protobuf_struct_pb.Struct | undefined;
  setMatch(value?: google_protobuf_struct_pb.Struct): void;

  hasDecide(): boolean;
  clearDecide(): void;
  getDecide(): google_protobuf_struct_pb.Struct | undefined;
  setDecide(value?: google_protobuf_struct_pb.Struct): void;

  getDescription(): string;
  setDescription(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Rule.AsObject;
  static toObject(includeInstance: boolean, msg: Rule): Rule.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Rule, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Rule;
  static deserializeBinaryFromReader(message: Rule, reader: jspb.BinaryReader): Rule;
}

export namespace Rule {
  export type AsObject = {
    id: string,
    name: string,
    type: RuleTypeMap[keyof RuleTypeMap],
    scope?: RuleScope.AsObject,
    status: RuleStatusMap[keyof RuleStatusMap],
    version: string,
    audit?: AuditMetadata.AsObject,
    match?: google_protobuf_struct_pb.Struct.AsObject,
    decide?: google_protobuf_struct_pb.Struct.AsObject,
    description: string,
  }
}

export class TraceStep extends jspb.Message {
  getRuleId(): string;
  setRuleId(value: string): void;

  getBundleId(): string;
  setBundleId(value: string): void;

  getDecisionType(): cordum_agent_v1_safety_pb.DecisionTypeMap[keyof cordum_agent_v1_safety_pb.DecisionTypeMap];
  setDecisionType(value: cordum_agent_v1_safety_pb.DecisionTypeMap[keyof cordum_agent_v1_safety_pb.DecisionTypeMap]): void;

  getReason(): string;
  setReason(value: string): void;

  hasTimestamp(): boolean;
  clearTimestamp(): void;
  getTimestamp(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setTimestamp(value?: google_protobuf_timestamp_pb.Timestamp): void;

  hasConstraints(): boolean;
  clearConstraints(): void;
  getConstraints(): google_protobuf_struct_pb.Struct | undefined;
  setConstraints(value?: google_protobuf_struct_pb.Struct): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): TraceStep.AsObject;
  static toObject(includeInstance: boolean, msg: TraceStep): TraceStep.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: TraceStep, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): TraceStep;
  static deserializeBinaryFromReader(message: TraceStep, reader: jspb.BinaryReader): TraceStep;
}

export namespace TraceStep {
  export type AsObject = {
    ruleId: string,
    bundleId: string,
    decisionType: cordum_agent_v1_safety_pb.DecisionTypeMap[keyof cordum_agent_v1_safety_pb.DecisionTypeMap],
    reason: string,
    timestamp?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    constraints?: google_protobuf_struct_pb.Struct.AsObject,
  }
}

export class Decision extends jspb.Message {
  getSource(): DecisionSourceMap[keyof DecisionSourceMap];
  setSource(value: DecisionSourceMap[keyof DecisionSourceMap]): void;

  getRuleId(): string;
  setRuleId(value: string): void;

  getBundleId(): string;
  setBundleId(value: string): void;

  getBundleVersion(): string;
  setBundleVersion(value: string): void;

  getType(): cordum_agent_v1_safety_pb.DecisionTypeMap[keyof cordum_agent_v1_safety_pb.DecisionTypeMap];
  setType(value: cordum_agent_v1_safety_pb.DecisionTypeMap[keyof cordum_agent_v1_safety_pb.DecisionTypeMap]): void;

  clearTraceList(): void;
  getTraceList(): Array<TraceStep>;
  setTraceList(value: Array<TraceStep>): void;
  addTrace(value?: TraceStep, index?: number): TraceStep;

  getInputRef(): string;
  setInputRef(value: string): void;

  getOutputRef(): string;
  setOutputRef(value: string): void;

  getAuditHash(): string;
  setAuditHash(value: string): void;

  hasTimestamp(): boolean;
  clearTimestamp(): void;
  getTimestamp(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setTimestamp(value?: google_protobuf_timestamp_pb.Timestamp): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Decision.AsObject;
  static toObject(includeInstance: boolean, msg: Decision): Decision.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Decision, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Decision;
  static deserializeBinaryFromReader(message: Decision, reader: jspb.BinaryReader): Decision;
}

export namespace Decision {
  export type AsObject = {
    source: DecisionSourceMap[keyof DecisionSourceMap],
    ruleId: string,
    bundleId: string,
    bundleVersion: string,
    type: cordum_agent_v1_safety_pb.DecisionTypeMap[keyof cordum_agent_v1_safety_pb.DecisionTypeMap],
    traceList: Array<TraceStep.AsObject>,
    inputRef: string,
    outputRef: string,
    auditHash: string,
    timestamp?: google_protobuf_timestamp_pb.Timestamp.AsObject,
  }
}

export class BundleMetadata extends jspb.Message {
  getEdgeMode(): EdgeModeMap[keyof EdgeModeMap];
  setEdgeMode(value: EdgeModeMap[keyof EdgeModeMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BundleMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: BundleMetadata): BundleMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BundleMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BundleMetadata;
  static deserializeBinaryFromReader(message: BundleMetadata, reader: jspb.BinaryReader): BundleMetadata;
}

export namespace BundleMetadata {
  export type AsObject = {
    edgeMode: EdgeModeMap[keyof EdgeModeMap],
  }
}

export class BundleVersion extends jspb.Message {
  getVersion(): string;
  setVersion(value: string): void;

  clearRuleSnapshotList(): void;
  getRuleSnapshotList(): Array<Rule>;
  setRuleSnapshotList(value: Array<Rule>): void;
  addRuleSnapshot(value?: Rule, index?: number): Rule;

  hasDeployedAt(): boolean;
  clearDeployedAt(): void;
  getDeployedAt(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setDeployedAt(value?: google_protobuf_timestamp_pb.Timestamp): void;

  getAuditHash(): string;
  setAuditHash(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BundleVersion.AsObject;
  static toObject(includeInstance: boolean, msg: BundleVersion): BundleVersion.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BundleVersion, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BundleVersion;
  static deserializeBinaryFromReader(message: BundleVersion, reader: jspb.BinaryReader): BundleVersion;
}

export namespace BundleVersion {
  export type AsObject = {
    version: string,
    ruleSnapshotList: Array<Rule.AsObject>,
    deployedAt?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    auditHash: string,
  }
}

export class Bundle extends jspb.Message {
  getId(): string;
  setId(value: string): void;

  getName(): string;
  setName(value: string): void;

  clearRuleIdsList(): void;
  getRuleIdsList(): Array<string>;
  setRuleIdsList(value: Array<string>): void;
  addRuleIds(value: string, index?: number): string;

  hasScopeBinding(): boolean;
  clearScopeBinding(): void;
  getScopeBinding(): RuleScope | undefined;
  setScopeBinding(value?: RuleScope): void;

  clearVersionsList(): void;
  getVersionsList(): Array<BundleVersion>;
  setVersionsList(value: Array<BundleVersion>): void;
  addVersions(value?: BundleVersion, index?: number): BundleVersion;

  hasMetadata(): boolean;
  clearMetadata(): void;
  getMetadata(): BundleMetadata | undefined;
  setMetadata(value?: BundleMetadata): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Bundle.AsObject;
  static toObject(includeInstance: boolean, msg: Bundle): Bundle.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Bundle, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Bundle;
  static deserializeBinaryFromReader(message: Bundle, reader: jspb.BinaryReader): Bundle;
}

export namespace Bundle {
  export type AsObject = {
    id: string,
    name: string,
    ruleIdsList: Array<string>,
    scopeBinding?: RuleScope.AsObject,
    versionsList: Array<BundleVersion.AsObject>,
    metadata?: BundleMetadata.AsObject,
  }
}

export class JobEvaluationContext extends jspb.Message {
  getTenantId(): string;
  setTenantId(value: string): void;

  getJobId(): string;
  setJobId(value: string): void;

  getWorkflowId(): string;
  setWorkflowId(value: string): void;

  getTopic(): string;
  setTopic(value: string): void;

  getPrincipalId(): string;
  setPrincipalId(value: string): void;

  getLabelsMap(): jspb.Map<string, string>;
  clearLabelsMap(): void;
  getMemoryId(): string;
  setMemoryId(value: string): void;

  getCapability(): string;
  setCapability(value: string): void;

  clearRiskTagsList(): void;
  getRiskTagsList(): Array<string>;
  setRiskTagsList(value: Array<string>): void;
  addRiskTags(value: string, index?: number): string;

  getInputContent(): Uint8Array | string;
  getInputContent_asU8(): Uint8Array;
  getInputContent_asB64(): string;
  setInputContent(value: Uint8Array | string): void;

  getInputContentType(): string;
  setInputContentType(value: string): void;

  getInputSizeBytes(): number;
  setInputSizeBytes(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JobEvaluationContext.AsObject;
  static toObject(includeInstance: boolean, msg: JobEvaluationContext): JobEvaluationContext.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JobEvaluationContext, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JobEvaluationContext;
  static deserializeBinaryFromReader(message: JobEvaluationContext, reader: jspb.BinaryReader): JobEvaluationContext;
}

export namespace JobEvaluationContext {
  export type AsObject = {
    tenantId: string,
    jobId: string,
    workflowId: string,
    topic: string,
    principalId: string,
    labelsMap: Array<[string, string]>,
    memoryId: string,
    capability: string,
    riskTagsList: Array<string>,
    inputContent: Uint8Array | string,
    inputContentType: string,
    inputSizeBytes: number,
  }
}

export class EdgeEvaluationContext extends jspb.Message {
  getTenantId(): string;
  setTenantId(value: string): void;

  getPrincipalId(): string;
  setPrincipalId(value: string): void;

  getSessionId(): string;
  setSessionId(value: string): void;

  getExecutionId(): string;
  setExecutionId(value: string): void;

  getAgentProduct(): string;
  setAgentProduct(value: string): void;

  getToolName(): string;
  setToolName(value: string): void;

  hasToolInputRedacted(): boolean;
  clearToolInputRedacted(): void;
  getToolInputRedacted(): google_protobuf_struct_pb.Struct | undefined;
  setToolInputRedacted(value?: google_protobuf_struct_pb.Struct): void;

  getInputHash(): string;
  setInputHash(value: string): void;

  getToolInputHash(): string;
  setToolInputHash(value: string): void;

  getLabelsMap(): jspb.Map<string, string>;
  clearLabelsMap(): void;
  clearRiskTagsList(): void;
  getRiskTagsList(): Array<string>;
  setRiskTagsList(value: Array<string>): void;
  addRiskTags(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): EdgeEvaluationContext.AsObject;
  static toObject(includeInstance: boolean, msg: EdgeEvaluationContext): EdgeEvaluationContext.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: EdgeEvaluationContext, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): EdgeEvaluationContext;
  static deserializeBinaryFromReader(message: EdgeEvaluationContext, reader: jspb.BinaryReader): EdgeEvaluationContext;
}

export namespace EdgeEvaluationContext {
  export type AsObject = {
    tenantId: string,
    principalId: string,
    sessionId: string,
    executionId: string,
    agentProduct: string,
    toolName: string,
    toolInputRedacted?: google_protobuf_struct_pb.Struct.AsObject,
    inputHash: string,
    toolInputHash: string,
    labelsMap: Array<[string, string]>,
    riskTagsList: Array<string>,
  }
}

export class PolicyEvaluateRequest extends jspb.Message {
  hasRule(): boolean;
  clearRule(): void;
  getRule(): Rule | undefined;
  setRule(value?: Rule): void;

  getBundleId(): string;
  setBundleId(value: string): void;

  hasScope(): boolean;
  clearScope(): void;
  getScope(): RuleScope | undefined;
  setScope(value?: RuleScope): void;

  hasJobContext(): boolean;
  clearJobContext(): void;
  getJobContext(): JobEvaluationContext | undefined;
  setJobContext(value?: JobEvaluationContext): void;

  hasEdgeContext(): boolean;
  clearEdgeContext(): void;
  getEdgeContext(): EdgeEvaluationContext | undefined;
  setEdgeContext(value?: EdgeEvaluationContext): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PolicyEvaluateRequest.AsObject;
  static toObject(includeInstance: boolean, msg: PolicyEvaluateRequest): PolicyEvaluateRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PolicyEvaluateRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PolicyEvaluateRequest;
  static deserializeBinaryFromReader(message: PolicyEvaluateRequest, reader: jspb.BinaryReader): PolicyEvaluateRequest;
}

export namespace PolicyEvaluateRequest {
  export type AsObject = {
    rule?: Rule.AsObject,
    bundleId: string,
    scope?: RuleScope.AsObject,
    jobContext?: JobEvaluationContext.AsObject,
    edgeContext?: EdgeEvaluationContext.AsObject,
  }
}

export class PolicyEvaluateResponse extends jspb.Message {
  hasDecision(): boolean;
  clearDecision(): void;
  getDecision(): Decision | undefined;
  setDecision(value?: Decision): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PolicyEvaluateResponse.AsObject;
  static toObject(includeInstance: boolean, msg: PolicyEvaluateResponse): PolicyEvaluateResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PolicyEvaluateResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PolicyEvaluateResponse;
  static deserializeBinaryFromReader(message: PolicyEvaluateResponse, reader: jspb.BinaryReader): PolicyEvaluateResponse;
}

export namespace PolicyEvaluateResponse {
  export type AsObject = {
    decision?: Decision.AsObject,
  }
}

export interface RuleTypeMap {
  RULE_TYPE_UNSPECIFIED: 0;
  RULE_TYPE_INPUT: 1;
  RULE_TYPE_OUTPUT: 2;
  RULE_TYPE_VELOCITY: 3;
  RULE_TYPE_EDGE: 4;
}

export const RuleType: RuleTypeMap;

export interface RuleStatusMap {
  RULE_STATUS_UNSPECIFIED: 0;
  RULE_STATUS_DRAFT: 1;
  RULE_STATUS_PUBLISHED: 2;
  RULE_STATUS_DEPRECATED: 3;
}

export const RuleStatus: RuleStatusMap;

export interface DecisionSourceMap {
  DECISION_SOURCE_UNSPECIFIED: 0;
  DECISION_SOURCE_JOB: 1;
  DECISION_SOURCE_EDGE: 2;
}

export const DecisionSource: DecisionSourceMap;

export interface RuleScopeKindMap {
  RULE_SCOPE_KIND_UNSPECIFIED: 0;
  RULE_SCOPE_KIND_GLOBAL: 1;
  RULE_SCOPE_KIND_TENANT: 2;
  RULE_SCOPE_KIND_WORKFLOW: 3;
  RULE_SCOPE_KIND_EDGE_FLEET: 4;
  RULE_SCOPE_KIND_EDGE_USER: 5;
}

export const RuleScopeKind: RuleScopeKindMap;

export interface EdgeModeMap {
  EDGE_MODE_UNSPECIFIED: 0;
  EDGE_MODE_OBSERVE: 1;
  EDGE_MODE_ENFORCE: 2;
  EDGE_MODE_ENTERPRISE_STRICT: 3;
}

export const EdgeMode: EdgeModeMap;

