import { Root, loadSync } from "protobufjs";
import path from "path";

const PROTO_BASE_DIR = path.resolve(__dirname, "..", "..", "..", "..", "proto");
const PROTOS = [
  "alert.proto",
  "buspacket.proto",
  "heartbeat.proto",
  "job.proto",
  "safety.proto",
].map((f) => path.join(PROTO_BASE_DIR, "coretex", "agent", "v1", f));

let root: Root | null = null;

export async function loadRoot(): Promise<Root> {
  if (root) {
    return root;
  }
  const protoRoot = new Root();
  protoRoot.resolvePath = (_origin, target) => {
    if (target.startsWith("coretex/")) {
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
export const DEFAULT_PROTOCOL_VERSION = 1;
