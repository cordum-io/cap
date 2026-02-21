package io.cordum.cap;

import io.cordum.cap.agent.v1.JobRequest;
import io.cordum.cap.agent.v1.JobResult;
import io.cordum.cap.agent.v1.JobStatus;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class WorkerTest {

    @Test
    void handlerInvocation() throws Exception {
        JobRequest req = JobRequest.newBuilder()
                .setJobId("job-1").setTopic("test").build();

        JobResult result = Testing.submitAndWait((ctx, r) ->
                JobResult.newBuilder()
                        .setJobId(r.getJobId())
                        .setStatus(JobStatus.JOB_STATUS_SUCCEEDED)
                        .build(), req);

        assertEquals("job-1", result.getJobId());
        assertEquals(JobStatus.JOB_STATUS_SUCCEEDED, result.getStatus());
    }

    @Test
    void middlewareOrdering() throws Exception {
        List<Integer> order = new ArrayList<>();

        Middleware.Interceptor mw1 = next -> (ctx, req) -> {
            order.add(1);
            return next.handle(ctx, req);
        };
        Middleware.Interceptor mw2 = next -> (ctx, req) -> {
            order.add(2);
            return next.handle(ctx, req);
        };

        Middleware.Handler base = (ctx, req) -> {
            order.add(3);
            return JobResult.newBuilder()
                    .setJobId(req.getJobId())
                    .setStatus(JobStatus.JOB_STATUS_SUCCEEDED)
                    .build();
        };

        Middleware.Handler wrapped = Middleware.applyMiddleware(base, List.of(mw1, mw2));

        JobRequest req = JobRequest.newBuilder().setJobId("j").setTopic("t").build();
        wrapped.handle(new Middleware.Context("t", "s", null), req);

        assertEquals(List.of(1, 2, 3), order);
    }

    @Test
    void emptyMiddlewareListReturnsBase() throws Exception {
        Middleware.Handler base = (ctx, req) ->
                JobResult.newBuilder()
                        .setJobId(req.getJobId())
                        .setStatus(JobStatus.JOB_STATUS_SUCCEEDED)
                        .build();

        Middleware.Handler wrapped = Middleware.applyMiddleware(base, List.of());

        JobRequest req = JobRequest.newBuilder().setJobId("j-1").setTopic("t").build();
        JobResult result = wrapped.handle(new Middleware.Context("t", "s", null), req);
        assertEquals("j-1", result.getJobId());
    }

    @Test
    void contextFieldsPopulated() throws Exception {
        final String[] captured = new String[2];

        Middleware.Handler handler = (ctx, req) -> {
            captured[0] = ctx.traceId();
            captured[1] = ctx.senderId();
            return JobResult.newBuilder()
                    .setJobId(req.getJobId())
                    .setStatus(JobStatus.JOB_STATUS_SUCCEEDED)
                    .build();
        };

        JobRequest req = JobRequest.newBuilder().setJobId("j").setTopic("t").build();
        handler.handle(new Middleware.Context("my-trace", "my-sender", null), req);

        assertEquals("my-trace", captured[0]);
        assertEquals("my-sender", captured[1]);
    }
}
