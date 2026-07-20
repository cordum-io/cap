/**
 * Opt-in validation helpers for CAP protobuf messages.
 *
 * Each function accepts a decoded protobufjs plain object and returns
 * an array of validation errors. An empty array means the message is valid.
 */

/** A single validation failure returned by the `validate*` functions. */
export interface ValidationError {
  /** Protobuf field path that failed validation (e.g. `"job_id"`). */
  field: string;
  /** Human-readable description of the failure. */
  message: string;
}

// Max known JobPriority enum value.
const JOB_PRIORITY_MAX = 3; // JOB_PRIORITY_CRITICAL

// JobStatus UNSPECIFIED value.
const JOB_STATUS_UNSPECIFIED = 0;
const SUPPORTED_PROTOCOL_VERSION = 1;
type MessageView = Record<string, unknown>;

function asMessage(value: unknown): MessageView | undefined {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? value as MessageView
    : undefined;
}

function field(message: MessageView, camel: string, snake?: string): unknown {
  return message[camel] ?? (snake ? message[snake] : undefined);
}

function textField(message: MessageView, camel: string, snake?: string): string {
  const value = field(message, camel, snake);
  return typeof value === "string" ? value : "";
}

function snakeName(value: string): string {
  return value.replace(/[A-Z]/g, (letter) => `_${letter.toLowerCase()}`);
}

function identityError(
  packet: MessageView,
  payloadName: string,
  identityName: string,
  path: string
): ValidationError[] {
  const payload = asMessage(field(packet, payloadName, snakeName(payloadName)));
  if (!payload) return [];
  const sender = textField(packet, "senderId", "sender_id");
  const nested = textField(payload, identityName, snakeName(identityName));
  return nested === sender ? [] : [{ field: path, message: "must equal sender_id" }];
}

function payloadPresent(message: MessageView): boolean {
  const names = [
    "jobRequest", "job_request", "jobResult", "job_result", "heartbeat",
    "alert", "jobProgress", "job_progress", "jobCancel", "job_cancel",
    "handshake", "workerHandshakeChallengeRequest", "worker_handshake_challenge_request",
    "workerHandshakeChallenge", "worker_handshake_challenge",
    "workerHandshakeAuthenticate", "worker_handshake_authenticate",
    "workerHandshakeResult", "worker_handshake_result",
  ];
  return names.some((name) => message[name] != null);
}

function alertIdentityError(message: MessageView): ValidationError[] {
  const alert = asMessage(field(message, "alert"));
  if (!alert) return [];
  const details = asMessage(field(alert, "details"));
  const structuredSignal = Boolean(
    field(alert, "severity") ||
    field(alert, "errorCodeEnum", "error_code_enum") ||
    field(alert, "sourceComponent", "source_component") ||
    (details && Object.keys(details).length > 0) ||
    textField(alert, "traceId", "trace_id")
  );
  const legacySignal = Boolean(
    field(alert, "level") || field(alert, "component") || field(alert, "code")
  );
  return structuredSignal || !legacySignal
    ? identityError(message, "alert", "sourceComponent", "alert.source_component")
    : [];
}

function validateNestedIdentity(message: MessageView): ValidationError[] {
  return [
    ...identityError(message, "jobResult", "workerId", "job_result.worker_id"),
    ...identityError(message, "heartbeat", "workerId", "heartbeat.worker_id"),
    ...identityError(message, "handshake", "componentId", "handshake.component_id"),
    ...alertIdentityError(message),
  ];
}

function validateBudget(value: unknown): ValidationError[] {
  const budget = asMessage(value);
  if (!budget) return [];
  const fields = [
    ["maxInputTokens", "max_input_tokens"],
    ["maxOutputTokens", "max_output_tokens"],
    ["maxTotalTokens", "max_total_tokens"],
    ["deadlineMs", "deadline_ms"],
  ] as const;
  return fields.flatMap(([camel, snake]) => {
    const amount = field(budget, camel, snake);
    return typeof amount === "number" && amount < 0
      ? [{ field: `budget.${snake}`, message: "must not be negative" }]
      : [];
  });
}

/**
 * Validates a decoded JobRequest message.
 */
export function validateJobRequest(msg: any): ValidationError[] {
  const errors: ValidationError[] = [];
  if (msg == null) {
    errors.push({ field: "JobRequest", message: "must not be nil" });
    return errors;
  }
  if (!msg.jobId && !msg.job_id) {
    errors.push({ field: "job_id", message: "must not be empty" });
  }
  if (!msg.topic) {
    errors.push({ field: "topic", message: "must not be empty" });
  }
  const priority = msg.priority ?? 0;
  if (priority < 0 || priority > JOB_PRIORITY_MAX) {
    errors.push({
      field: "priority",
      message: `invalid value ${priority}`,
    });
  }
  const stepIndex = msg.stepIndex ?? msg.step_index ?? 0;
  if (stepIndex < 0) {
    errors.push({ field: "step_index", message: "must not be negative" });
  }
  errors.push(...validateBudget(msg.budget));
  return errors;
}

/**
 * Validates a decoded JobResult message.
 */
export function validateJobResult(msg: any): ValidationError[] {
  const errors: ValidationError[] = [];
  if (msg == null) {
    errors.push({ field: "JobResult", message: "must not be nil" });
    return errors;
  }
  if (!msg.jobId && !msg.job_id) {
    errors.push({ field: "job_id", message: "must not be empty" });
  }
  const status = msg.status ?? 0;
  if (status === JOB_STATUS_UNSPECIFIED) {
    errors.push({ field: "status", message: "must not be UNSPECIFIED" });
  }
  if (!msg.workerId && !msg.worker_id) {
    errors.push({ field: "worker_id", message: "must not be empty" });
  }
  const executionMs = msg.executionMs ?? msg.execution_ms ?? 0;
  if (executionMs < 0) {
    errors.push({
      field: "execution_ms",
      message: "must not be negative",
    });
  }
  return errors;
}

/**
 * Validates a decoded BusPacket message.
 * If the packet carries a JobRequest or JobResult payload, it is also validated.
 */
export function validateBusPacket(value: unknown): ValidationError[] {
  const errors: ValidationError[] = [];
  const msg = asMessage(value);
  if (!msg) {
    errors.push({ field: "BusPacket", message: "must not be nil" });
    return errors;
  }
  if (!textField(msg, "traceId", "trace_id")) {
    errors.push({ field: "trace_id", message: "must not be empty" });
  }
  if (!textField(msg, "senderId", "sender_id")) {
    errors.push({ field: "sender_id", message: "must not be empty" });
  }
  const protocolVersion = field(msg, "protocolVersion", "protocol_version");
  if (protocolVersion !== SUPPORTED_PROTOCOL_VERSION) {
    errors.push({ field: "protocol_version", message: "must equal 1" });
  }
  if (field(msg, "createdAt", "created_at") == null) {
    errors.push({ field: "created_at", message: "must not be nil" });
  }
  if (!payloadPresent(msg)) {
    errors.push({ field: "payload", message: "must not be nil" });
  }
  const jobReq = field(msg, "jobRequest", "job_request");
  if (jobReq != null) {
    errors.push(...validateJobRequest(jobReq));
  }
  const jobRes = field(msg, "jobResult", "job_result");
  if (jobRes != null) {
    errors.push(...validateJobResult(jobRes));
  }
  errors.push(...validateNestedIdentity(msg));
  return errors;
}
