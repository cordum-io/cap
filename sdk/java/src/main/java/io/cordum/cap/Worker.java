package io.cordum.cap;

import com.google.protobuf.InvalidProtocolBufferException;
import com.google.protobuf.Timestamp;
import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.JobRequest;
import io.cordum.cap.agent.v1.JobResult;
import io.cordum.cap.agent.v1.JobStatus;
import io.nats.client.Connection;
import io.nats.client.Dispatcher;

import java.security.GeneralSecurityException;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

public class Worker {

    private final Connection nc;
    private final String subject;
    private final String senderId;
    private Middleware.Handler handler;
    private final List<Middleware.Interceptor> middlewares = new ArrayList<>();
    private PrivateKey privateKey;
    private Map<String, PublicKey> publicKeys = Map.of();
    private Metrics metrics = Metrics.NOOP;
    private Dispatcher dispatcher;

    public Worker(Connection nc, String subject, String senderId,
                  Middleware.Handler handler) {
        this.nc = nc;
        this.subject = subject;
        this.senderId = senderId;
        this.handler = handler;
    }

    public Worker privateKey(PrivateKey key) { this.privateKey = key; return this; }
    public Worker publicKeys(Map<String, PublicKey> keys) { this.publicKeys = keys; return this; }
    public Worker metrics(Metrics metrics) { this.metrics = metrics; return this; }

    public Worker use(Middleware.Interceptor... mw) {
        for (Middleware.Interceptor m : mw) {
            middlewares.add(m);
        }
        return this;
    }

    public void start() {
        Middleware.Handler wrapped = Middleware.applyMiddleware(handler, middlewares);

        dispatcher = nc.createDispatcher(msg -> {
            BusPacket packet;
            try {
                packet = BusPacket.parseFrom(msg.getData());
            } catch (InvalidProtocolBufferException e) {
                metrics.onError("parse", "failed to parse BusPacket: " + e.getMessage());
                return;
            }

            if (!publicKeys.isEmpty()) {
                String sender = packet.getSenderId();
                PublicKey key = publicKeys.get(sender);
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

            long start = System.currentTimeMillis();
            JobResult result;
            try {
                result = wrapped.handle(ctx, req);
            } catch (Exception e) {
                result = JobResult.newBuilder()
                        .setJobId(req.getJobId())
                        .setWorkerId(senderId)
                        .setStatus(JobStatus.JOB_STATUS_FAILED)
                        .setErrorMessage(e.getMessage() != null ? e.getMessage() : "handler exception")
                        .build();
            }
            long elapsed = System.currentTimeMillis() - start;

            if (result.getJobId().isEmpty()) {
                result = result.toBuilder().setJobId(req.getJobId()).build();
            }
            if (result.getWorkerId().isEmpty()) {
                result = result.toBuilder().setWorkerId(senderId).build();
            }

            if (result.getStatus() == JobStatus.JOB_STATUS_FAILED) {
                metrics.onJobFailed(req.getJobId(), result.getErrorMessage());
            } else {
                metrics.onJobCompleted(req.getJobId(), elapsed,
                        result.getStatus().name());
            }

            long now = System.currentTimeMillis();
            Timestamp ts = Timestamp.newBuilder()
                    .setSeconds(now / 1000)
                    .setNanos((int) ((now % 1000) * 1_000_000))
                    .build();

            BusPacket out = BusPacket.newBuilder()
                    .setTraceId(packet.getTraceId())
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

            byte[] data = Codec.marshalDeterministic(out);
            nc.publish(Subjects.RESULT, data);
        });
        dispatcher.subscribe(subject, subject);
    }

    public void stop() {
        if (dispatcher != null) {
            nc.closeDispatcher(dispatcher);
            dispatcher = null;
        }
    }
}
