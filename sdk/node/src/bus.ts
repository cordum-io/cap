import { connect, NatsConnection } from "nats";

/** NATS connection configuration. */
export interface NATSConfig {
  url: string;
  token?: string;
  user?: string;
  pass?: string;
  /** Client name reported to the NATS server. Defaults to `"cap-sdk-node"`. */
  name?: string;
}

/**
 * Opens a NATS connection using the provided configuration.
 *
 * @param cfg - Connection settings.
 * @returns A connected NATS client.
 */
export async function connectNATS(cfg: NATSConfig): Promise<NatsConnection> {
  return connect({
    servers: cfg.url,
    token: cfg.token,
    user: cfg.user,
    pass: cfg.pass,
    name: cfg.name ?? "cap-sdk-node",
  });
}
