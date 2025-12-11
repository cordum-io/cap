import { connect, NatsConnection } from "nats";

export interface NATSConfig {
  url: string;
  token?: string;
  user?: string;
  pass?: string;
  name?: string;
}

export async function connectNATS(cfg: NATSConfig): Promise<NatsConnection> {
  return connect({
    servers: cfg.url,
    token: cfg.token,
    user: cfg.user,
    pass: cfg.pass,
    name: cfg.name ?? "cap-sdk-node",
  });
}
