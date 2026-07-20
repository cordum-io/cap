const { randomUUID } = require("node:crypto");
const {
  connectNATS,
  DEFAULT_PROTOCOL_VERSION,
  loadRoot,
  SUBJECT_RESULT,
  validateBusPacket,
} = require("cap-sdk-node");

const JOB_SUBJECT = "job.echo";
const TERMINAL_FAILURES = new Set([
  "JOB_STATUS_FAILED",
  "JOB_STATUS_CANCELLED",
  "JOB_STATUS_DENIED",
  "JOB_STATUS_TIMEOUT",
  "JOB_STATUS_FAILED_RETRYABLE",
  "JOB_STATUS_FAILED_FATAL",
]);

function resultTimeoutMs() {
  const raw = process.env.CAP_RESULT_TIMEOUT_SECONDS ?? "15";
  const seconds = Number(raw);
  if (!Number.isFinite(seconds) || seconds <= 0) {
    throw new Error("CAP_RESULT_TIMEOUT_SECONDS must be a positive number");
  }
  return Math.max(1, Math.floor(seconds * 1000));
}

async function loadTypes() {
  const root = await loadRoot();
  return {
    busPacket: root.lookupType("cordum.agent.v1.BusPacket"),
    jobRequest: root.lookupType("cordum.agent.v1.JobRequest"),
    priorities: root.lookupEnum("cordum.agent.v1.JobPriority"),
    statuses: root.lookupEnum("cordum.agent.v1.JobStatus"),
  };
}

function timestampNow() {
  const milliseconds = Date.now();
  return {
    seconds: Math.floor(milliseconds / 1000),
    nanos: (milliseconds % 1000) * 1_000_000,
  };
}

function requireDirectSubject(topic) {
  const hasEmptyToken =
    typeof topic === "string" &&
    topic.split(".").some((token) => token.length === 0);
  if (
    typeof topic !== "string" ||
    topic.length === 0 ||
    hasEmptyToken ||
    /[\s*>]/u.test(topic)
  ) {
    throw new Error("direct JobRequest topic must be a concrete NATS subject");
  }
  return topic;
}

function buildRequest(types, jobId) {
  return types.jobRequest.create({
    jobId,
    topic: JOB_SUBJECT,
    priority: types.priorities.values.JOB_PRIORITY_INTERACTIVE,
    contextPtr: `demo://context/${jobId}`,
  });
}

function buildPacket(types, req, traceId) {
  requireDirectSubject(req.topic);
  const packet = types.busPacket.create({
    traceId,
    senderId: "simple-echo-node-client",
    createdAt: timestampNow(),
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    jobRequest: req,
  });
  const errors = validateBusPacket(packet);
  if (errors.length > 0) {
    throw new Error(`invalid JobRequest packet: ${JSON.stringify(errors)}`);
  }
  return packet;
}

function inspectResult(types, data, traceId, jobId) {
  let packet;
  try {
    packet = types.busPacket.decode(data);
  } catch {
    return { kind: "ignore" };
  }
  const result = packet.jobResult;
  if (!result || packet.traceId !== traceId || result.jobId !== jobId) {
    return { kind: "ignore" };
  }
  const errors = validateBusPacket(packet);
  if (errors.length > 0) {
    return {
      kind: "error",
      message: `invalid matching result: ${JSON.stringify(errors)}`,
    };
  }
  const status = types.statuses.valuesById[result.status];
  if (status === "JOB_STATUS_SUCCEEDED") return { kind: "success" };
  if (!status || TERMINAL_FAILURES.has(status)) {
    const detail = status ?? `unknown status ${result.status}`;
    return { kind: "error", message: `job ended with ${detail}` };
  }
  return { kind: "ignore" };
}

function subscribeForResult(nc, types, traceId, jobId, timeoutMs) {
  let settled = false;
  let timer;
  let resolveDone;
  let rejectDone;
  const done = new Promise((resolve, reject) => {
    resolveDone = resolve;
    rejectDone = reject;
  });
  void done.catch(() => undefined);
  const finish = (error) => {
    if (settled) return;
    settled = true;
    clearTimeout(timer);
    if (error) rejectDone(error);
    else resolveDone();
  };
  timer = setTimeout(
    () => finish(new Error(`timed out after ${timeoutMs}ms waiting for JobResult`)),
    timeoutMs
  );
  let subscription;
  try {
    subscription = nc.subscribe(SUBJECT_RESULT, {
      callback: (error, message) => {
        if (error) return finish(error);
        try {
          const verdict = inspectResult(types, message.data, traceId, jobId);
          if (verdict.kind === "success") finish();
          if (verdict.kind === "error") finish(new Error(verdict.message));
        } catch (inspectionError) {
          finish(inspectionError);
        }
      },
    });
  } catch (error) {
    finish(error);
    throw error;
  }
  const cancel = () => {
    if (settled) return;
    settled = true;
    clearTimeout(timer);
  };
  return { cancel, done, subscription };
}

async function cleanup(nc, observer, primaryError) {
  observer?.cancel();
  const cleanupErrors = [];
  try {
    observer?.subscription.unsubscribe();
  } catch (error) {
    cleanupErrors.push(error);
  }
  try {
    await nc.drain();
  } catch (error) {
    cleanupErrors.push(error);
  }
  if (cleanupErrors.length === 0) return;
  if (!primaryError) throw cleanupErrors[0];
  for (const error of cleanupErrors) {
    console.error(
      `secondary NATS cleanup error: ${
        error instanceof Error ? error.message : String(error)
      }`
    );
  }
}

async function main() {
  const nc = await connectNATS({
    url: process.env.CAP_NATS_URL ?? "nats://127.0.0.1:4222",
    name: "cap-simple-echo-node-client",
  });
  let observer;
  let primaryError;
  try {
    const types = await loadTypes();
    const jobId = `job-${randomUUID()}`;
    const traceId = `trace-${randomUUID()}`;
    const req = buildRequest(types, jobId);
    const packet = buildPacket(types, req, traceId);
    observer = subscribeForResult(nc, types, traceId, jobId, resultTimeoutMs());
    await nc.flush();

    // DEVELOPMENT ONLY: publishes to the worker pool without platform governance.
    console.warn(
      `DEV-ONLY direct publish to ${req.topic}; bypasses Gateway, Scheduler, ` +
        "Safety Kernel, policy, and authentication"
    );
    nc.publish(req.topic, types.busPacket.encode(packet).finish());
    await nc.flush();
    await observer.done;
    console.log(`CAP_SIMPLE_ECHO_SUCCESS job_id=${jobId} trace_id=${traceId}`);
  } catch (error) {
    primaryError = error;
    throw error;
  } finally {
    await cleanup(nc, observer, primaryError);
  }
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
});
