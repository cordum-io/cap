package io.cordum.cap;

import com.google.protobuf.InvalidProtocolBufferException;
import com.google.protobuf.Timestamp;
import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.JobRequest;
import io.cordum.cap.agent.v1.JobResult;
import io.cordum.cap.agent.v1.JobStatus;
import io.nats.client.Connection;
import io.nats.client.Dispatcher;

import java.io.IOException;
import java.security.GeneralSecurityException;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;

public class Agent implements AutoCloseable {

    private final Connection nc;
    private final String senderId;
    private final String pool;
    private final int maxParallel;
    private final PrivateKey privateKey;
    private final Map<String, PublicKey> publicKeys;
    private final Metrics metrics;
    private final List<Middleware.Interceptor> middlewares;
    private final long heartbeatIntervalMs;

    private final Map<String, Middleware.Handler> handlers = new HashMap<>();
    private final List<Dispatcher> dispatchers = new ArrayList<>();
    private Heartbeat.HeartbeatLoop heartbeatLoop;
    private final AtomicInteger activeJobs = new AtomicInteger(0);
    private boolean started;

    private Agent(Builder builder) throws IOException, InterruptedException {
        if (builder.nc != null) {
            this.nc = builder.nc;
        } else {
            Bus.Config busConfig = new Bus.Config().url(builder.natsUrl);
            this.nc = Bus.connect(busConfig);
        }
        this.senderId = builder.senderId;
        this.pool = builder.pool;
        this.maxParallel = builder.maxParallel;
        this.privateKey = builder.privateKey;
        this.publicKeys = builder.publicKeys;
        this.metrics = builder.metrics;
        this.middlewares = builder.middlewares;
        this.heartbeatIntervalMs = builder.heartbeatIntervalMs;
    }

    public void registerHandler(String subject, Middleware.Handler handler) {
        handlers.put(subject, handler);
    }

    public void start() {
        if (started) return;

        for (Map.Entry<String, Middleware.Handler> entry : handlers.entrySet()) {
            String subject = entry.getKey();
            Middleware.Handler wrapped = Middleware.applyMiddleware(entry.getValue(), middlewares);

            Dispatcher dispatcher = nc.createDispatcher(msg -> {
                onMessage(subject, wrapped, msg.getData());
            });
            dispatcher.subscribe(subject, subject);
            dispatchers.add(dispatcher);
        }

        heartbeatLoop = new Heartbeat.HeartbeatLoop(
                nc,
                () -> Heartbeat.heartbeatPayload(
                        senderId, pool,
                        activeJobs.get(), maxParallel, 0f),
                senderId,
                heartbeatIntervalMs,
                metrics);

        started = true;
    }

    @Override
    public void close() {
        if (!started) return;
        if (heartbeatLoop != null) {
            heartbeatLoop.stop();
            heartbeatLoop = null;
        }
        for (Dispatcher d : dispatchers) {
            nc.closeDispatcher(d);
        }
        dispatchers.clear();
        started = false;
    }

    private void onMessage(String subject, Middleware.Handler handler, byte[] data) {
        BusPacket packet;
        try {
            packet = BusPacket.parseFrom(data);
        } catch (InvalidProtocolBufferException e) {
            metrics.onError("parse", "failed to parse BusPacket: " + e.getMessage());
            return;
        }

        if (!publicKeys.isEmpty()) {
            PublicKey key = publicKeys.get(packet.getSenderId());
            if (key == null) return;
            if (packet.getSignature().isEmpty()) return;
            try {
                if (!Signing.verifyPacketSignature(packet, key)) return;
            } catch (GeneralSecurityException e) {
                return;
            }
        }

        if (packet.getPayloadCase() != BusPacket.PayloadCase.JOB_REQUEST) return;

        JobRequest req = packet.getJobRequest();
        metrics.onJobReceived(req.getJobId(), req.getTopic());

        Middleware.Context ctx = new Middleware.Context(
                packet.getTraceId(), packet.getSenderId(), packet);

        activeJobs.incrementAndGet();
        long startTime = System.currentTimeMillis();

        JobResult result;
        try {
            result = handler.handle(ctx, req);
        } catch (Exception e) {
            activeJobs.decrementAndGet();
            metrics.onJobFailed(req.getJobId(), e.getMessage());
            result = JobResult.newBuilder()
                    .setJobId(req.getJobId())
                    .setWorkerId(senderId)
                    .setStatus(JobStatus.JOB_STATUS_FAILED)
                    .setErrorMessage(e.getMessage() != null ? e.getMessage() : "handler exception")
                    .build();
            publishResult(packet, result);
            return;
        }

        activeJobs.decrementAndGet();
        long elapsed = System.currentTimeMillis() - startTime;

        if (result.getJobId().isEmpty()) {
            result = result.toBuilder().setJobId(req.getJobId()).build();
        }
        if (result.getWorkerId().isEmpty()) {
            result = result.toBuilder().setWorkerId(senderId).build();
        }

        if (result.getStatus() == JobStatus.JOB_STATUS_FAILED) {
            metrics.onJobFailed(req.getJobId(), result.getErrorMessage());
        } else {
            metrics.onJobCompleted(req.getJobId(), elapsed, result.getStatus().name());
        }

        publishResult(packet, result);
    }

    private void publishResult(BusPacket inbound, JobResult result) {
        long now = System.currentTimeMillis();
        Timestamp ts = Timestamp.newBuilder()
                .setSeconds(now / 1000)
                .setNanos((int) ((now % 1000) * 1_000_000))
                .build();

        BusPacket out = BusPacket.newBuilder()
                .setTraceId(inbound.getTraceId())
                .setSenderId(senderId)
                .setProtocolVersion(Subjects.DEFAULT_PROTOCOL_VERSION)
                .setCreatedAt(ts)
                .setJobResult(result)
                .build();

        if (privateKey != null) {
            try {
                out = Signing.signPacket(out, privateKey);
            } catch (GeneralSecurityException e) {
                metrics.onError("signing", "failed to sign outgoing packet");
                return;
            }
        }

        nc.publish(Subjects.RESULT, Codec.marshalDeterministic(out));
    }

    public static class Builder {
        private Connection nc;
        private String natsUrl = "nats://localhost:4222";
        private String senderId;
        private String pool = "";
        private int maxParallel = 1;
        private PrivateKey privateKey;
        private Map<String, PublicKey> publicKeys = Map.of();
        private Metrics metrics = Metrics.NOOP;
        private List<Middleware.Interceptor> middlewares = new ArrayList<>();
        private long heartbeatIntervalMs = Subjects.DEFAULT_HEARTBEAT_INTERVAL_MS;

        public Builder connection(Connection nc) { this.nc = nc; return this; }
        public Builder natsUrl(String url) { this.natsUrl = url; return this; }
        public Builder senderId(String id) { this.senderId = id; return this; }
        public Builder pool(String pool) { this.pool = pool; return this; }
        public Builder maxParallel(int n) { this.maxParallel = n; return this; }
        public Builder privateKey(PrivateKey key) { this.privateKey = key; return this; }
        public Builder publicKeys(Map<String, PublicKey> keys) { this.publicKeys = keys; return this; }
        public Builder metrics(Metrics metrics) { this.metrics = metrics; return this; }
        public Builder use(Middleware.Interceptor mw) { this.middlewares.add(mw); return this; }
        public Builder heartbeatIntervalMs(long ms) { this.heartbeatIntervalMs = ms; return this; }

        public Agent build() throws IOException, InterruptedException {
            if (senderId == null || senderId.isEmpty()) {
                throw new IllegalArgumentException("senderId is required");
            }
            return new Agent(this);
        }
    }
}
