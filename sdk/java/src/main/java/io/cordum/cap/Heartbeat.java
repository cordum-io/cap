package io.cordum.cap;

import com.google.protobuf.Timestamp;
import io.cordum.cap.agent.v1.BusPacket;
import io.nats.client.Connection;

import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;

public final class Heartbeat {

    private Heartbeat() {}

    public static byte[] heartbeatPayload(String workerId, String pool,
                                           int activeJobs, int maxParallel, float cpuLoad) {
        return heartbeatPayloadWithProgress(workerId, pool, activeJobs, maxParallel,
                cpuLoad, 0f, 0, "");
    }

    public static byte[] heartbeatPayloadWithMemory(String workerId, String pool,
                                                     int activeJobs, int maxParallel,
                                                     float cpuLoad, float memoryLoad) {
        return heartbeatPayloadWithProgress(workerId, pool, activeJobs, maxParallel,
                cpuLoad, memoryLoad, 0, "");
    }

    public static byte[] heartbeatPayloadWithProgress(String workerId, String pool,
                                                       int activeJobs, int maxParallel,
                                                       float cpuLoad, float memoryLoad,
                                                       int progressPct, String lastMemo) {
        long now = System.currentTimeMillis();
        Timestamp ts = Timestamp.newBuilder()
                .setSeconds(now / 1000)
                .setNanos((int) ((now % 1000) * 1_000_000))
                .build();

        io.cordum.cap.agent.v1.Heartbeat hb = io.cordum.cap.agent.v1.Heartbeat.newBuilder()
                .setWorkerId(workerId)
                .setPool(pool)
                .setActiveJobs(activeJobs)
                .setMaxParallelJobs(maxParallel)
                .setCpuLoad(cpuLoad)
                .setMemoryLoad(memoryLoad)
                .setProgressPct(progressPct)
                .setLastMemo(lastMemo)
                .build();

        BusPacket packet = BusPacket.newBuilder()
                .setSenderId(workerId)
                .setProtocolVersion(Subjects.DEFAULT_PROTOCOL_VERSION)
                .setCreatedAt(ts)
                .setHeartbeat(hb)
                .build();

        return Codec.marshalDeterministic(packet);
    }

    public static void emitHeartbeat(Connection nc, byte[] payload) {
        nc.publish(Subjects.HEARTBEAT, payload);
    }

    /** Periodically emits heartbeats using a ScheduledExecutorService. */
    public static class HeartbeatLoop {
        private final Connection nc;
        private final Supplier<byte[]> payloadFn;
        private final String workerId;
        private final Metrics metrics;
        private final ScheduledExecutorService executor;

        public HeartbeatLoop(Connection nc, Supplier<byte[]> payloadFn,
                             String workerId, long intervalMs, Metrics metrics) {
            this.nc = nc;
            this.payloadFn = payloadFn;
            this.workerId = workerId;
            this.metrics = metrics;
            this.executor = Executors.newSingleThreadScheduledExecutor(r -> {
                Thread t = new Thread(r, "cap-heartbeat");
                t.setDaemon(true);
                return t;
            });
            executor.scheduleAtFixedRate(this::tick, 0, intervalMs, TimeUnit.MILLISECONDS);
        }

        private void tick() {
            try {
                byte[] payload = payloadFn.get();
                emitHeartbeat(nc, payload);
                metrics.onHeartbeatSent(workerId);
            } catch (Exception e) {
                metrics.onError("heartbeat", e.getMessage());
            }
        }

        public void stop() {
            executor.shutdownNow();
        }
    }
}
