import {
  WORKER_HANDSHAKE_MAX_SKEW_MS,
  ProtoTimestamp,
  WorkerHandshakeExpiredError,
  dateNanos,
  timestampNanos,
} from "./worker-trust-contract";
import { WorkerHandshakeChallenge } from "./worker-trust-types";

export function requireLiveChallenge(
  challenge: WorkerHandshakeChallenge,
  now: Date
): void {
  requireFreshTimestamp(challenge.issuedAt, now, "challenge issued_at");
  if (timestampNanos(challenge.expiresAt) <= dateNanos(now)) {
    throw new WorkerHandshakeExpiredError("challenge is expired");
  }
}

export function requireFreshTimestamp(
  value: ProtoTimestamp | undefined,
  now: Date,
  field: string
): void {
  const at = timestampNanos(value);
  const nowNanos = dateNanos(now);
  const difference = at >= nowNanos ? at - nowNanos : nowNanos - at;
  if (difference > BigInt(WORKER_HANDSHAKE_MAX_SKEW_MS) * 1_000_000n) {
    throw new WorkerHandshakeExpiredError(`${field} exceeds allowed skew`);
  }
}
