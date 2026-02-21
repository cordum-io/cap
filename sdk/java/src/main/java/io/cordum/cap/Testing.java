package io.cordum.cap;

import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.JobRequest;
import io.cordum.cap.agent.v1.JobResult;
import io.cordum.cap.agent.v1.JobStatus;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.function.Consumer;

public final class Testing {

    private Testing() {}

    /** Tracks published messages for assertions. */
    public static class Published {
        public final String subject;
        public final byte[] data;

        public Published(String subject, byte[] data) {
            this.subject = subject;
            this.data = data;
        }
    }

    /** In-memory mock for NATS-like publish/subscribe for unit tests. */
    public static class MockBus {
        private final CopyOnWriteArrayList<Published> published = new CopyOnWriteArrayList<>();
        private final ConcurrentHashMap<String, List<Consumer<byte[]>>> subscribers = new ConcurrentHashMap<>();

        public void publish(String subject, byte[] data) {
            published.add(new Published(subject, data));
            List<Consumer<byte[]>> handlers = subscribers.get(subject);
            if (handlers != null) {
                for (Consumer<byte[]> h : handlers) {
                    h.accept(data);
                }
            }
        }

        public void subscribe(String subject, Consumer<byte[]> handler) {
            subscribers.computeIfAbsent(subject, k -> new CopyOnWriteArrayList<>()).add(handler);
        }

        public List<Published> published() {
            return new ArrayList<>(published);
        }

        public List<Published> publishedTo(String subject) {
            List<Published> result = new ArrayList<>();
            for (Published p : published) {
                if (p.subject.equals(subject)) {
                    result.add(p);
                }
            }
            return result;
        }

        public void clear() {
            published.clear();
            subscribers.clear();
        }
    }

    /**
     * Submit a job request to a handler and return the result.
     * Useful for quick unit tests without NATS infrastructure.
     */
    public static JobResult submitAndWait(Middleware.Handler handler, JobRequest req) throws Exception {
        Middleware.Context ctx = new Middleware.Context("test-trace", "test-sender", null);
        return handler.handle(ctx, req);
    }

    /** Recording implementation of Metrics for test assertions. */
    public static class RecordingMetrics implements Metrics {
        public final List<String> receivedJobs = new CopyOnWriteArrayList<>();
        public final List<String> completedJobs = new CopyOnWriteArrayList<>();
        public final List<String> failedJobs = new CopyOnWriteArrayList<>();
        public final List<String> heartbeats = new CopyOnWriteArrayList<>();
        public final List<String> errors = new CopyOnWriteArrayList<>();
        public final List<String> progress = new CopyOnWriteArrayList<>();

        @Override public void onJobReceived(String jobId, String topic) { receivedJobs.add(jobId); }
        @Override public void onJobCompleted(String jobId, long durationMs, String status) { completedJobs.add(jobId); }
        @Override public void onJobFailed(String jobId, String errorMsg) { failedJobs.add(jobId); }
        @Override public void onHeartbeatSent(String workerId) { heartbeats.add(workerId); }
        @Override public void onProgressEmitted(String jobId, int percent) { progress.add(jobId); }
        @Override public void onError(String context, String errorMsg) { errors.add(context + ": " + errorMsg); }
    }
}
