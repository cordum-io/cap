package io.cordum.cap;

import io.cordum.cap.agent.v1.JobRequest;
import io.cordum.cap.agent.v1.JobResult;
import io.cordum.cap.agent.v1.JobStatus;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class AgentTest {

    @Test
    void submitAndWaitBasicHandler() throws Exception {
        JobRequest req = JobRequest.newBuilder()
                .setJobId("j-1").setTopic("echo").build();

        JobResult result = Testing.submitAndWait((ctx, r) ->
                JobResult.newBuilder()
                        .setJobId(r.getJobId())
                        .setWorkerId("w-1")
                        .setStatus(JobStatus.JOB_STATUS_SUCCEEDED)
                        .build(), req);

        assertEquals("j-1", result.getJobId());
        assertEquals("w-1", result.getWorkerId());
        assertEquals(JobStatus.JOB_STATUS_SUCCEEDED, result.getStatus());
    }

    @Test
    void handlerExceptionReturnsError() {
        JobRequest req = JobRequest.newBuilder()
                .setJobId("j-1").setTopic("fail").build();

        assertThrows(RuntimeException.class, () ->
                Testing.submitAndWait((ctx, r) -> {
                    throw new RuntimeException("boom");
                }, req));
    }

    @Test
    void recordingMetricsTracksEvents() {
        Testing.RecordingMetrics metrics = new Testing.RecordingMetrics();

        metrics.onJobReceived("j-1", "test");
        metrics.onJobCompleted("j-1", 100, "SUCCEEDED");
        metrics.onHeartbeatSent("w-1");
        metrics.onError("parse", "bad data");

        assertEquals(1, metrics.receivedJobs.size());
        assertEquals("j-1", metrics.receivedJobs.get(0));
        assertEquals(1, metrics.completedJobs.size());
        assertEquals(1, metrics.heartbeats.size());
        assertEquals(1, metrics.errors.size());
        assertTrue(metrics.errors.get(0).contains("bad data"));
    }

    @Test
    void mockBusTracksPublishes() {
        Testing.MockBus bus = new Testing.MockBus();

        bus.publish("sys.heartbeat", new byte[]{1, 2, 3});
        bus.publish("sys.job.result", new byte[]{4, 5});

        assertEquals(2, bus.published().size());
        assertEquals(1, bus.publishedTo("sys.heartbeat").size());
        assertEquals(1, bus.publishedTo("sys.job.result").size());
        assertEquals(0, bus.publishedTo("sys.job.submit").size());
    }

    @Test
    void mockBusDeliversToSubscribers() {
        Testing.MockBus bus = new Testing.MockBus();
        List<byte[]> received = new ArrayList<>();

        bus.subscribe("test.subject", received::add);
        bus.publish("test.subject", new byte[]{1, 2, 3});

        assertEquals(1, received.size());
        assertArrayEquals(new byte[]{1, 2, 3}, received.get(0));
    }

    @Test
    void middlewareChainInAgent() throws Exception {
        List<String> log = new ArrayList<>();

        Middleware.Interceptor mw = next -> (ctx, req) -> {
            log.add("mw-before");
            JobResult r = next.handle(ctx, req);
            log.add("mw-after");
            return r;
        };

        Middleware.Handler base = (ctx, req) -> {
            log.add("handler");
            return JobResult.newBuilder()
                    .setJobId(req.getJobId())
                    .setStatus(JobStatus.JOB_STATUS_SUCCEEDED)
                    .build();
        };

        Middleware.Handler wrapped = Middleware.applyMiddleware(base, List.of(mw));
        JobRequest req = JobRequest.newBuilder().setJobId("j").setTopic("t").build();
        wrapped.handle(new Middleware.Context("t", "s", null), req);

        assertEquals(List.of("mw-before", "handler", "mw-after"), log);
    }
}
