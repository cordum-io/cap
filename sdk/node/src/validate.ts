/**
 * Opt-in validation helpers for CAP protobuf messages.
 *
 * Each function accepts a decoded protobufjs plain object and returns
 * an array of validation errors. An empty array means the message is valid.
 */

export interface ValidationError {
  field: string;
  message: string;
}

// Max known JobPriority enum value.
const JOB_PRIORITY_MAX = 3; // JOB_PRIORITY_CRITICAL

// JobStatus UNSPECIFIED value.
const JOB_STATUS_UNSPECIFIED = 0;

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
  const budget = msg.budget;
  if (budget != null) {
    if ((budget.maxInputTokens ?? budget.max_input_tokens ?? 0) < 0) {
      errors.push({
        field: "budget.max_input_tokens",
        message: "must not be negative",
      });
    }
    if ((budget.maxOutputTokens ?? budget.max_output_tokens ?? 0) < 0) {
      errors.push({
        field: "budget.max_output_tokens",
        message: "must not be negative",
      });
    }
    if ((budget.maxTotalTokens ?? budget.max_total_tokens ?? 0) < 0) {
      errors.push({
        field: "budget.max_total_tokens",
        message: "must not be negative",
      });
    }
    if ((budget.deadlineMs ?? budget.deadline_ms ?? 0) < 0) {
      errors.push({
        field: "budget.deadline_ms",
        message: "must not be negative",
      });
    }
  }
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
export function validateBusPacket(msg: any): ValidationError[] {
  const errors: ValidationError[] = [];
  if (msg == null) {
    errors.push({ field: "BusPacket", message: "must not be nil" });
    return errors;
  }
  if (!msg.traceId && !msg.trace_id) {
    errors.push({ field: "trace_id", message: "must not be empty" });
  }
  if (!msg.senderId && !msg.sender_id) {
    errors.push({ field: "sender_id", message: "must not be empty" });
  }
  const protocolVersion =
    msg.protocolVersion ?? msg.protocol_version ?? 0;
  if (protocolVersion <= 0) {
    errors.push({ field: "protocol_version", message: "must be > 0" });
  }
  if (msg.createdAt == null && msg.created_at == null) {
    errors.push({ field: "created_at", message: "must not be nil" });
  }
  const hasPayload =
    msg.jobRequest != null ||
    msg.job_request != null ||
    msg.jobResult != null ||
    msg.job_result != null ||
    msg.heartbeat != null ||
    msg.alert != null ||
    msg.jobProgress != null ||
    msg.job_progress != null ||
    msg.jobCancel != null ||
    msg.job_cancel != null ||
    msg.handshake != null;
  if (!hasPayload) {
    errors.push({ field: "payload", message: "must not be nil" });
  }
  // Delegate to payload-specific validation.
  const jobReq = msg.jobRequest ?? msg.job_request;
  if (jobReq != null) {
    errors.push(...validateJobRequest(jobReq));
  }
  const jobRes = msg.jobResult ?? msg.job_result;
  if (jobRes != null) {
    errors.push(...validateJobResult(jobRes));
  }
  return errors;
}
