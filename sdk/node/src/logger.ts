/**
 * Structured logger interface for CAP SDK.
 * All log methods accept a message and optional structured fields.
 * The default `console` object satisfies this interface.
 */
export interface Logger {
  info(msg: string, fields?: Record<string, any>): void;
  warn(msg: string, fields?: Record<string, any>): void;
  error(msg: string, fields?: Record<string, any>): void;
}

/**
 * Default structured logger that outputs JSON lines.
 * Uses stdout for info, stderr for warn and error.
 */
export const defaultLogger: Logger = {
  info(msg: string, fields?: Record<string, any>): void {
    console.info(JSON.stringify({ level: "info", msg, ...fields, ts: new Date().toISOString() }));
  },
  warn(msg: string, fields?: Record<string, any>): void {
    console.warn(JSON.stringify({ level: "warn", msg, ...fields, ts: new Date().toISOString() }));
  },
  error(msg: string, fields?: Record<string, any>): void {
    console.error(JSON.stringify({ level: "error", msg, ...fields, ts: new Date().toISOString() }));
  },
};
