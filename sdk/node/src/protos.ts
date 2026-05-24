import { Root, loadSync } from "protobufjs";
import fs from "fs";
import path from "path";

const PROTO_DIR_CANDIDATES = [
  path.resolve(__dirname, "..", "..", "..", "proto"),
  path.resolve(__dirname, "..", "..", "..", "..", "proto"),
];
const PROTO_BASE_DIR =
  PROTO_DIR_CANDIDATES.find((candidate) => fs.existsSync(candidate)) ??
  PROTO_DIR_CANDIDATES[0];
const PROTOS = [
  "alert.proto",
  "buspacket.proto",
  "handshake.proto",
  "heartbeat.proto",
  "job.proto",
  "policy.proto",
  "safety.proto",
].map((f) => path.join(PROTO_BASE_DIR, "cordum", "agent", "v1", f));

let root: Root | null = null;

export async function loadRoot(): Promise<Root> {
  if (root) {
    return root;
  }
  const protoRoot = new Root();
  protoRoot.resolvePath = (_origin, target) => {
    if (target.startsWith("cordum/")) {
      return path.join(PROTO_BASE_DIR, target);
    }
    return path.resolve(path.dirname(_origin), target);
  };
  root = protoRoot.loadSync(PROTOS);
  return root;
}

export const SUBJECT_SUBMIT = "sys.job.submit";
export const SUBJECT_RESULT = "sys.job.result";
export const SUBJECT_HEARTBEAT = "sys.heartbeat";
export const SUBJECT_ALERT = "sys.alert";
export const SUBJECT_PROGRESS = "sys.job.progress";
export const SUBJECT_CANCEL = "sys.job.cancel";
export const SUBJECT_DLQ = "sys.job.dlq";
export const SUBJECT_WORKFLOW_EVENT = "sys.workflow.event";
export const SUBJECT_HANDSHAKE = "sys.handshake";
export const DEFAULT_PROTOCOL_VERSION = 1;

/** Maximum length (in characters) of a human-facing agent display label. */
export const MAX_AGENT_NAME_LEN = 128;

/**
 * Trim, collapse internal whitespace, and bound a human-facing agent display
 * label for safe transport on Heartbeat/Handshake. The result is a DISPLAY
 * label only: consumers MUST prefer authenticated identity records over this
 * self-reported value and MUST NOT treat it as proof of identity.
 */
export function sanitizeAgentName(name: string): string {
  return name.replace(/\s+/g, " ").trim().slice(0, MAX_AGENT_NAME_LEN);
}
