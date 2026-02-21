package io.cordum.cap;

import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.JobRequest;
import io.cordum.cap.agent.v1.JobResult;

import java.util.List;
import java.util.logging.Logger;

public final class Middleware {

    private Middleware() {}

    /** Context passed through the middleware chain. */
    public static final class Context {
        private final String traceId;
        private final String senderId;
        private final BusPacket packet;

        public Context(String traceId, String senderId, BusPacket packet) {
            this.traceId = traceId;
            this.senderId = senderId;
            this.packet = packet;
        }

        public String traceId() { return traceId; }
        public String senderId() { return senderId; }
        public BusPacket packet() { return packet; }
    }

    /** Processes a job request and returns a result. */
    @FunctionalInterface
    public interface Handler {
        JobResult handle(Context ctx, JobRequest request) throws Exception;
    }

    /** Wraps a handler, returning a new handler. */
    @FunctionalInterface
    public interface Interceptor {
        Handler apply(Handler next);
    }

    /**
     * Compose a chain of interceptors around a base handler.
     * Interceptors execute in FIFO order (first added = outermost wrapper).
     */
    public static Handler applyMiddleware(Handler base, List<Interceptor> chain) {
        Handler handler = base;
        for (int i = chain.size() - 1; i >= 0; i--) {
            handler = chain.get(i).apply(handler);
        }
        return handler;
    }

    /** Interceptor that logs job_id and duration for each request. */
    public static Interceptor loggingMiddleware(Logger logger) {
        return next -> (ctx, req) -> {
            long start = System.currentTimeMillis();
            JobResult result = next.handle(ctx, req);
            long elapsed = System.currentTimeMillis() - start;
            logger.info("[cap] job=" + req.getJobId() + " duration=" + elapsed + "ms");
            return result;
        };
    }
}
