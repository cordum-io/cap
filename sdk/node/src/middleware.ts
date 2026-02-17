import type { Context } from "./runtime";

/**
 * Middleware intercepts handler execution. Call `next()` to invoke the
 * next middleware or the terminal handler. Middleware is applied in FIFO
 * order: the first registered middleware is the outermost.
 */
export type Middleware = (ctx: Context, next: () => Promise<any>) => Promise<any>;

/**
 * Returns a middleware that logs job ID, topic, and duration for every request.
 *
 * @param logger - Logger to use. Defaults to `console`.
 */
export function loggingMiddleware(logger: { info: (...args: any[]) => void } = console): Middleware {
  return async (ctx: Context, next: () => Promise<any>): Promise<any> => {
    const start = Date.now();
    try {
      const result = await next();
      const elapsed = Date.now() - start;
      logger.info(`middleware: job=${ctx.jobId} topic=${ctx.job?.topic ?? "?"} duration=${elapsed}ms ok`);
      return result;
    } catch (err) {
      const elapsed = Date.now() - start;
      logger.info(`middleware: job=${ctx.jobId} topic=${ctx.job?.topic ?? "?"} duration=${elapsed}ms error=${err}`);
      throw err;
    }
  };
}
