import { load } from "protobufjs";
import path from "path";

// dist is at sdk/node/dist; walk up to repo root then into proto/
const PROTO_DIR = path.resolve(__dirname, "../../../proto/cortex/agent/v1");

export async function loadRoot() {
  return load([
    path.join(PROTO_DIR, "buspacket.proto"),
    path.join(PROTO_DIR, "job.proto"),
    path.join(PROTO_DIR, "heartbeat.proto"),
    path.join(PROTO_DIR, "alert.proto"),
    path.join(PROTO_DIR, "safety.proto"),
  ]);
}

export const SUBJECT_SUBMIT = "sys.job.submit";
export const SUBJECT_RESULT = "sys.job.result";
export const SUBJECT_HEARTBEAT = "sys.heartbeat";
export const DEFAULT_PROTOCOL_VERSION = 1;
